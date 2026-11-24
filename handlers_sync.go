package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

import (
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/types"
	wsclient "todoapp/internal/websocket"
)

// conflictRecord 待记录的同步冲突（事务提交后统一落库，避免事务内跨连接写锁竞争）
type conflictRecord struct {
	localID  string
	serverID int64
	reason   string
}

// conflictOptions 返回冲突解决选项；删除/重复插入场景没有"合并"语义
func conflictOptions(includeMerge bool) []types.ConflictResolution {
	if includeMerge {
		return []types.ConflictResolution{types.ConflictKeepServer, types.ConflictKeepClient, types.ConflictMerge}
	}
	return []types.ConflictResolution{types.ConflictKeepServer, types.ConflictKeepClient}
}

// payloadString 从 delta payload 安全读取字符串字段
func payloadString(p map[string]interface{}, key string) (string, bool) {
	if v, ok := p[key].(string); ok {
		return v, true
	}
	return "", false
}

// collectTaskExtra 从 delta payload 提取"显式携带才更新"的扩展字段（手动排序/循环任务），
// 缺省字段由 UpdateTaskWithVersion 保持原值，避免旧客户端 payload 误清服务端已有配置
func collectTaskExtra(payload map[string]interface{}) *db.TaskExtraFields {
	extra := &db.TaskExtraFields{}
	if v, ok := payload["sort_order"].(float64); ok {
		sortOrder := v
		extra.SortOrder = &sortOrder
	}
	if v, ok := payload["recurrence_type"].(string); ok {
		// 非空且非法的规则直接丢弃（保持服务端原值），避免脏数据入库；空串表示清除循环，允许
		if v == "" || db.IsValidRecurrenceRule(v) {
			recurrenceType := v
			extra.RecurrenceType = &recurrenceType
		}
	}
	if v, ok := payload["recurrence_series_id"].(string); ok {
		series := v
		extra.RecurrenceSeriesID = &series
	}
	return extra
}

// afterTaskUpdate 处理任务更新后的循环任务衍生逻辑：
// - 客户端设置了循环规则但未带系列 ID 时，服务端补齐
// - 状态从非 done 迁移到 done 时，物化下一次 occurrence（失败仅记录日志，不阻断同步主体）
func afterTaskUpdate(q *sql.Tx, userID int, taskID int64, oldStatus, newStatus string, extra *db.TaskExtraFields) {
	if extra != nil && extra.RecurrenceType != nil && db.IsValidRecurrenceRule(*extra.RecurrenceType) {
		if _, err := db.EnsureRecurrenceSeriesID(q, userID, taskID); err != nil {
			log.Printf("补齐循环系列 ID 失败: task_id=%d %v", taskID, err)
		}
	}
	if newStatus == "done" && oldStatus != "done" {
		if err := db.MaterializeNextTx(q, userID, taskID); err != nil {
			log.Printf("生成循环任务下一次失败: task_id=%d %v", taskID, err)
		}
	}
}

func handleSync(w http.ResponseWriter, r *http.Request, wsHub *wsclient.Hub) {
	var s syncReq
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		response.ErrorResponse(w, "错误的请求", http.StatusBadRequest)
		return
	}

	// 从上下文获取用户 ID（已认证的用户）
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户 ID", http.StatusUnauthorized)
		return
	}

	now := time.Now().UTC()

	// ✅ 使用事务保护同步操作：所有 insert/update/delete 均在事务内执行，任一失败整体回滚
	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("事务开始失败: %v", err)
		response.ErrorResponse(w, "事务开始失败", http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	serverChanges := []map[string]interface{}{}
	clientChanges := []map[string]interface{}{}
	conflicts := []map[string]interface{}{}
	pendingConflicts := []conflictRecord{}

	// syncFail 同步过程中任一变更失败：整体回滚并返回 500，客户端保留 delta 队列稍后重试
	syncFail := func(errLog string, err error) {
		log.Printf("%s: %v", errLog, err)
		response.ErrorResponse(w, "同步失败，请稍后重试", http.StatusInternalServerError)
	}

	for _, c := range s.Changes {
		op := strings.ToLower(c.Op)

		switch op {
		case "insert":
			title := ""
			description := ""
			status := "todo"
			priority := "medium"

			if v, ok := c.Payload["title"].(string); ok {
				title = v
			}
			if v, ok := c.Payload["description"].(string); ok {
				description = v
			}
			if v, ok := c.Payload["status"].(string); ok {
				status = v
			}
			if v, ok := c.Payload["priority"].(string); ok {
				priority = v
			}

			// 修复：插入路径必须读取 due_at，否则安卓端带截止日期的新任务会丢失 due_at；
			// 同时透传手动排序与循环任务字段
			dueAt, hasDueAt := payloadString(c.Payload, "due_at")
			dueAtArg := interface{}(nil)
			if hasDueAt && dueAt != "" {
				dueAtArg = dueAt
			}
			sortOrderVal := 0.0
			if v, ok := c.Payload["sort_order"].(float64); ok {
				sortOrderVal = v
			}
			recurrenceType, hasRecurrence := payloadString(c.Payload, "recurrence_type")
			recurrenceSeriesID, _ := payloadString(c.Payload, "recurrence_series_id")
			if hasRecurrence && db.IsValidRecurrenceRule(recurrenceType) {
				if recurrenceSeriesID == "" {
					recurrenceSeriesID = db.NewSeriesID()
				}
			} else {
				recurrenceType = ""
				recurrenceSeriesID = ""
			}

			// ✅ 检查 local_id 是否已存在（冲突检测）
			existingID, _, checkErr := db.TaskExistsByLocalID(tx, userID, c.LocalID)
			if checkErr != nil && !errors.Is(checkErr, sql.ErrNoRows) {
				syncFail("检查 local_id 失败", checkErr)
				return
			}
			if checkErr == nil && existingID > 0 {
				// 冲突：local_id重复，使用生成的新标题插入
				newTitle := fmt.Sprintf("%s (副本: %d)", title, existingID)
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, due_at, created_at, updated_at, last_modified, sort_order, recurrence_type, recurrence_series_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID+"_"+randomString(8), 1, newTitle, description, status, priority, dueAtArg, now, now, now, sortOrderVal, recurrenceType, recurrenceSeriesID,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						// 附带既有任务的当前版本号，供客户端"保留本地"时作为 CV 重放
						existVer, _, _, _, _, _, verErr := db.GetTaskForSync(tx, userID, existingID)
						if verErr != nil {
							existVer = 0
						}
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": newTitle, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
							"due_at": dueAtArg, "sort_order": sortOrderVal,
							"recurrence_type": recurrenceType, "recurrence_series_id": recurrenceSeriesID,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})

						// 记录冲突（事务提交后落库）
						pendingConflicts = append(pendingConflicts, conflictRecord{localID: c.LocalID, serverID: existingID, reason: "duplicate_insert"})
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":       c.LocalID,
							"server_id":      existingID,
							"reason":         "duplicate_insert",
							"server_version": existVer,
							"options":        conflictOptions(false),
						})
					}
				} else {
					syncFail("插入副本失败", insertErr)
					return
				}
			} else {
				// 正常插入
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, due_at, created_at, updated_at, last_modified, sort_order, recurrence_type, recurrence_series_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID, 1, title, description, status, priority, dueAtArg, now, now, now, sortOrderVal, recurrenceType, recurrenceSeriesID,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
							"due_at": dueAtArg, "sort_order": sortOrderVal,
							"recurrence_type": recurrenceType, "recurrence_series_id": recurrenceSeriesID,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})
					}
				} else {
					syncFail("插入任务失败", insertErr)
					return
				}
			}

		case "update":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)

				// 客户端携带的截止日期（nil 表示本次不更新 due_at）
				var clientDueAt *string
				if v, ok := c.Payload["due_at"].(string); ok {
					clientDueAt = &v
				}

				// ✅ 获取服务器当前版本和数据（校验任务归属，防止越权修改他人任务）
				oldVer, serverTitle, serverDesc, serverStatus, serverPriority, _, queryErr := db.GetTaskForSync(tx, userID, id)

				if queryErr != nil {
					if errors.Is(queryErr, sql.ErrNoRows) {
						// 任务不存在（从未同步或已硬删）：回执 server_id=0 供客户端清理滞留 delta，不阻塞同批其他变更
						log.Printf("跳过不存在的任务 delta: local_id=%s id=%v", c.LocalID, idVal)
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": int64(0), "op": op,
						})
						continue
					}
					syncFail("查询任务失败", queryErr)
					return
				}

				newVer := oldVer + 1

				// ✅ 版本检查和冲突检测
				if c.CV != oldVer {
					log.Printf("检测到冲突: client_version=%d, server_version=%d", c.CV, oldVer)

					// 获取客户端更新（记录字段是否显式出现在 delta 中，用于字段级冲突明细）
					clientTitle := ""
					clientDesc := ""
					clientStatus := serverStatus
					hasTitle := false
					hasDesc := false

					if v, ok := c.Payload["title"].(string); ok {
						clientTitle = v
						hasTitle = true
					}
					if v, ok := c.Payload["description"].(string); ok {
						clientDesc = v
						hasDesc = true
					}
					if v, ok := c.Payload["status"].(string); ok {
						clientStatus = v
					}

					// ✅ 智能合并
					mergedTitle, mergedDesc, mergedStatus := intelligentMerge(serverTitle, serverDesc, serverStatus, map[string]interface{}{
						"title":       clientTitle,
						"description": clientDesc,
						"status":      clientStatus,
					})

					// 应用合并结果；priority 缺省保持服务端原值（拖拽排序等 delta 不携带该字段，禁止重置为默认值）
					priority := serverPriority
					if v, ok := c.Payload["priority"].(string); ok {
						priority = v
					}

					// 手动排序/循环字段：显式携带才更新，避免缺省 payload 误清已有配置
					extra := collectTaskExtra(c.Payload)

					updateErr := db.UpdateTaskWithVersion(tx, userID, id, mergedTitle, mergedDesc, mergedStatus, priority, clientDueAt, newVer, extra)
					if updateErr != nil {
						syncFail("应用合并结果失败", updateErr)
						return
					} else {
						afterTaskUpdate(tx, userID, id, serverStatus, mergedStatus, extra)
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": mergedTitle, "description": mergedDesc,
							"status": mergedStatus, "priority": priority, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})

						// 字段级冲突明细：仅列出客户端显式修改且与服务端不同的字段
						fieldConflicts := []types.FieldLevelConflict{}
						if hasTitle && clientTitle != serverTitle {
							fieldConflicts = append(fieldConflicts, types.FieldLevelConflict{FieldName: "title", ServerValue: serverTitle, ClientValue: clientTitle})
						}
						if hasDesc && clientDesc != serverDesc {
							fieldConflicts = append(fieldConflicts, types.FieldLevelConflict{FieldName: "description", ServerValue: serverDesc, ClientValue: clientDesc})
						}
						if clientStatus != serverStatus {
							fieldConflicts = append(fieldConflicts, types.FieldLevelConflict{FieldName: "status", ServerValue: serverStatus, ClientValue: clientStatus})
						}

						// 记录冲突（事务提交后落库）
						pendingConflicts = append(pendingConflicts, conflictRecord{localID: c.LocalID, serverID: id, reason: "intelligent_merge"})
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":       c.LocalID,
							"server_id":      id,
							"reason":         "intelligent_merge",
							"server_version": newVer,
							"resolution":     "智能合并",
							"options":        conflictOptions(true),
							"conflicts":      fieldConflicts,
							"merged_data": map[string]interface{}{
								"title":       mergedTitle,
								"description": mergedDesc,
								"status":      mergedStatus,
							},
						})
					}
				} else {
					// 正常更新
					title := serverTitle
					description := serverDesc
					status := serverStatus
					// priority 缺省保持服务端原值（拖拽排序等 delta 不携带该字段，禁止重置为默认值）
					priority := serverPriority

					if v, ok := c.Payload["title"].(string); ok {
						title = v
					}
					if v, ok := c.Payload["description"].(string); ok {
						description = v
					}
					if v, ok := c.Payload["status"].(string); ok {
						status = v
					}
					if v, ok := c.Payload["priority"].(string); ok {
						priority = v
					}

					// 手动排序/循环字段：显式携带才更新，避免缺省 payload 误清已有配置
					extra := collectTaskExtra(c.Payload)

					updateErr := db.UpdateTaskWithVersion(tx, userID, id, title, description, status, priority, clientDueAt, newVer, extra)
					if updateErr != nil {
						syncFail("更新任务失败", updateErr)
						return
					} else {
						afterTaskUpdate(tx, userID, id, serverStatus, status, extra)
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})
					}
				}
			} else {
				// payload 缺少服务端任务 ID（如本地 insert 尚未确认即产生更新）：
				// 回执给客户端以便其清理滞留 delta，避免重复重发
				clientChanges = append(clientChanges, map[string]interface{}{
					"local_id": c.LocalID, "server_id": int64(0), "op": op,
				})
			}

		case "delete":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)

				oldVer, _, _, _, _, isDeleted, queryErr := db.GetTaskForSync(tx, userID, id)

				if queryErr != nil {
					if errors.Is(queryErr, sql.ErrNoRows) {
						// 任务不存在（从未同步或已硬删）：回执 server_id=0 供客户端清理滞留 delta，不阻塞同批其他变更
						log.Printf("跳过不存在的任务删除 delta: local_id=%s id=%v", c.LocalID, idVal)
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": int64(0), "op": op,
						})
						continue
					}
					syncFail("查询任务失败", queryErr)
					return
				}

				newVer := oldVer + 1

				// ✅ 版本检查和冲突检测
				if c.CV != oldVer {
					log.Printf("删除冲突检测: client_version=%d, server_version=%d", c.CV, oldVer)

					if isDeleted {
						// 服务器已删除，忽略客户端删除
						continue
					} else {
						// 冲突：客户端想删除但服务器有更新
						// 策略：软删除，记录冲突
						deleteErr := db.SoftDeleteTaskWithVersion(tx, userID, id, newVer)
						if deleteErr != nil {
							syncFail("删除任务失败", deleteErr)
							return
						} else {
							serverChanges = append(serverChanges, map[string]interface{}{
								"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
							})
							clientChanges = append(clientChanges, map[string]interface{}{
								"local_id": c.LocalID, "server_id": id, "op": "delete",
							})

							// 记录冲突：服务器被标记为删除（事务提交后落库）
							pendingConflicts = append(pendingConflicts, conflictRecord{localID: c.LocalID, serverID: id, reason: "delete_while_modified"})
							conflicts = append(conflicts, map[string]interface{}{
								"local_id":       c.LocalID,
								"server_id":      id,
								"reason":         "delete_while_modified",
								"server_version": newVer,
								"options":        conflictOptions(false),
							})
						}
					}
				} else {
					// 正常删除
					deleteErr := db.SoftDeleteTaskWithVersion(tx, userID, id, newVer)
					if deleteErr != nil {
						syncFail("删除任务失败", deleteErr)
						return
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "delete",
						})
					}
				}
			} else {
				// payload 缺少服务端任务 ID：回执给客户端以便其清理滞留 delta
				clientChanges = append(clientChanges, map[string]interface{}{
					"local_id": c.LocalID, "server_id": int64(0), "op": op,
				})
			}
		}
	}

	// ✅ 显式提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("事务提交失败: %v", err)
		response.ErrorResponse(w, "同步失败", http.StatusInternalServerError)
		return
	}
	committed = true

	// 事务提交后统一记录冲突，避免事务内跨连接写 conflicts 表的锁竞争
	for _, pc := range pendingConflicts {
		recordConflict(pc.localID, pc.serverID, pc.reason, userID)
	}

	// 发送同步结果通知
	if len(conflicts) > 0 {
		_, _ = sendNotificationToUser(userID, "sync_conflict", "同步完成但存在冲突", fmt.Sprintf("成功同步 %d 个任务，但检测到 %d 个冲突", len(clientChanges), len(conflicts)), "high", wsHub)
	} else if len(clientChanges) > 0 {
		_, _ = sendNotificationToUser(userID, "sync_success", "同步完成", fmt.Sprintf("成功同步 %d 个任务", len(clientChanges)), "normal", wsHub)
	}

	// ✅ 增量拉取：携带 last_sync_at 游标时，返回该时间及之后服务端变更的任务（含软删除，供对端同步删除）
	if s.LastSyncAt != "" {
		if since, parseErr := time.Parse(time.RFC3339, s.LastSyncAt); parseErr == nil {
			changed, qErr := db.GetTasksChangedSince(userID, since)
			if qErr != nil {
				log.Printf("增量拉取失败: %v", qErr)
			} else {
				serverChanges = append(serverChanges, changed...)
			}
		} else {
			log.Printf("last_sync_at 游标解析失败，跳过增量拉取: %v", parseErr)
		}
	}

	resp := map[string]interface{}{
		"server_changes": serverChanges,
		"client_changes": clientChanges,
		"last_sync_at":   now.Format(time.RFC3339),
		"conflicts":      conflicts, // ✅ 返回实际冲突
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleInitPairing 初始化配对，请求服务器生成配对密钥
