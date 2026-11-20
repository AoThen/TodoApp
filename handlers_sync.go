package main

import (
	"encoding/json"
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
	wsclient "todoapp/internal/websocket"
)

// conflictRecord 待记录的同步冲突（事务提交后统一落库，避免事务内跨连接写锁竞争）
type conflictRecord struct {
	localID  string
	serverID int64
	reason   string
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

	// ✅ 使用事务保护同步操作
	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("事务开始失败: %v", err)
		response.ErrorResponse(w, "事务开始失败", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	serverChanges := []map[string]interface{}{}
	clientChanges := []map[string]interface{}{}
	conflicts := []map[string]interface{}{}
	pendingConflicts := []conflictRecord{}
	syncFailed := false

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

			// ✅ 检查 local_id 是否已存在（冲突检测）
			existingID, _, checkErr := db.TaskExistsByLocalID(userID, c.LocalID)
			if checkErr == nil && existingID > 0 {
				// 冲突：local_id重复，使用生成的新标题插入
				newTitle := fmt.Sprintf("%s (副本: %d)", title, existingID)
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID+"_"+randomString(8), 1, newTitle, description, status, priority, now, now, now,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": newTitle, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})

						// 记录冲突（事务提交后落库）
						pendingConflicts = append(pendingConflicts, conflictRecord{localID: c.LocalID, serverID: existingID, reason: "duplicate_insert"})
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":  c.LocalID,
							"server_id": existingID,
							"reason":    "duplicate_insert",
						})
					}
				} else {
					log.Printf("插入副本失败: %v", insertErr)
					syncFailed = true
				}
			} else {
				// 正常插入
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID, 1, title, description, status, priority, now, now, now,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})
					}
				} else {
					log.Printf("插入任务失败: %v", insertErr)
					syncFailed = true
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

				// ✅ 获取服务器当前版本和数据
				oldVer, serverTitle, serverDesc, serverStatus, _, err := db.GetTaskForSync(id)

				if err != nil {
					log.Printf("查询任务失败: %v", err)
					syncFailed = true
					continue
				}

				newVer := oldVer + 1

				// ✅ 版本检查和冲突检测
				if c.CV != oldVer {
					log.Printf("检测到冲突: client_version=%d, server_version=%d", c.CV, oldVer)

					// 获取客户端更新
					clientTitle := ""
					clientDesc := ""
					clientStatus := serverStatus

					if v, ok := c.Payload["title"].(string); ok {
						clientTitle = v
					}
					if v, ok := c.Payload["description"].(string); ok {
						clientDesc = v
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

					// 应用合并结果
					priority := "medium"
					if v, ok := c.Payload["priority"].(string); ok {
						priority = v
					}

					updateErr := db.UpdateTaskWithVersion(id, mergedTitle, mergedDesc, mergedStatus, priority, clientDueAt, newVer)
					if updateErr != nil {
						log.Printf("应用合并结果失败: %v", updateErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": mergedTitle, "description": mergedDesc,
							"status": mergedStatus, "priority": priority, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})

						// 记录冲突（事务提交后落库）
						pendingConflicts = append(pendingConflicts, conflictRecord{localID: c.LocalID, serverID: id, reason: "intelligent_merge"})
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":   c.LocalID,
							"server_id":  id,
							"reason":     "intelligent_merge",
							"resolution": "智能合并",
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

					updateErr := db.UpdateTaskWithVersion(id, title, description, status, priority, clientDueAt, newVer)
					if updateErr != nil {
						log.Printf("更新任务失败: %v", updateErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})
					}
				}
			}

		case "delete":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)

				oldVer, _, _, _, isDeleted, err := db.GetTaskForSync(id)

				if err != nil {
					log.Printf("查询任务失败: %v", err)
					syncFailed = true
					continue
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
						deleteErr := db.SoftDeleteTaskWithVersion(id, newVer)
						if deleteErr != nil {
							log.Printf("删除任务失败: %v", deleteErr)
							syncFailed = true
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
								"local_id":  c.LocalID,
								"server_id": id,
								"reason":    "delete_while_modified",
							})
						}
					}
				} else {
					// 正常删除
					deleteErr := db.SoftDeleteTaskWithVersion(id, newVer)
					if deleteErr != nil {
						log.Printf("删除任务失败: %v", deleteErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "delete",
						})
					}
				}
			}
		}
	}

	// ✅ 显式提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("事务提交失败: %v", err)
		response.ErrorResponse(w, "同步失败", http.StatusInternalServerError)
		return
	}

	// 事务提交后统一记录冲突，避免事务内跨连接写 conflicts 表的锁竞争
	for _, pc := range pendingConflicts {
		recordConflict(pc.localID, pc.serverID, pc.reason, userID)
	}

	// 发送同步结果通知
	if syncFailed {
		_, _ = sendNotificationToUser(userID, "sync_failed", "同步失败", "部分任务同步失败，请检查网络连接", "high", wsHub)
	} else if len(conflicts) > 0 {
		_, _ = sendNotificationToUser(userID, "sync_conflict", "同步完成但存在冲突", fmt.Sprintf("成功同步 %d 个任务，但检测到 %d 个冲突", len(clientChanges), len(conflicts)), "high", wsHub)
	} else if len(clientChanges) > 0 {
		_, _ = sendNotificationToUser(userID, "sync_success", "同步完成", fmt.Sprintf("成功同步 %d 个任务", len(clientChanges)), "normal", wsHub)
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
