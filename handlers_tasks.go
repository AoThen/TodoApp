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
	"github.com/gorilla/mux"
)

import (
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/types"
	"todoapp/internal/validator"
	wsclient "todoapp/internal/websocket"
)

func handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 从上下文获取用户 ID
		userIDStr := getUserIDFromContext(r.Context())
		userID := 0
		if userIDStr != "" {
			var err error
			userID, err = strconv.Atoi(userIDStr)
			if err != nil {
				log.Printf("转换用户ID错误: %v", err)
				userID = 0
			}
		}

		// 解析分页参数
		pageStr := r.URL.Query().Get("page")
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		pageSizeStr := r.URL.Query().Get("page_size")
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		// 解析筛选参数
		status := r.URL.Query().Get("status")
		priority := r.URL.Query().Get("priority")
		search := r.URL.Query().Get("search")
		sortBy := r.URL.Query().Get("sort_by")
		sortOrder := r.URL.Query().Get("sort_order")

		tasks, total, err := db.GetTasksFiltered(userID, page, pageSize, status, priority, search, sortBy, sortOrder)
		if err != nil {
			response.ErrorResponse(w, "获取任务失败", http.StatusInternalServerError)
			return
		}

		response.SuccessResponse(w, map[string]interface{}{
			"tasks": tasks,
			"pagination": map[string]interface{}{
				"page":      page,
				"page_size": pageSize,
				"total":     total,
				"pages":     (total + pageSize - 1) / pageSize,
			},
		}, http.StatusOK)
		return
	}

	// POST: 创建任务
	if r.Method == http.MethodPost {
		var t task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
			return
		}

		if t.Title == "" {
			response.ErrorResponse(w, "标题不能为空", http.StatusBadRequest)
			return
		}
		if !validator.IsValidTaskTitle(t.Title) {
			response.ErrorResponse(w, "标题长度需在1-200字符之间", http.StatusBadRequest)
			return
		}

		userIDStr := getUserIDFromContext(r.Context())
		userID := 0
		if userIDStr != "" {
			var err error
			userID, err = strconv.Atoi(userIDStr)
			if err != nil {
				response.ErrorResponse(w, "用户ID无效", http.StatusBadRequest)
				return
			}
		}

		if userID == 0 {
			response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
			return
		}

		taskID, err := db.CreateTask(userID, "", t.Title)
		if err != nil {
			log.Printf("创建任务失败: %v", err)
			response.ErrorResponse(w, "创建任务失败", http.StatusInternalServerError)
			return
		}

		t.ID = int(taskID)
		response.SuccessResponse(w, t, http.StatusCreated)
		return
	}

	response.ErrorResponse(w, "方法不允许", http.StatusMethodNotAllowed)
}

func handleTaskByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的任务ID", http.StatusBadRequest)
		return
	}

	userIDStr := getUserIDFromContext(r.Context())
	userID := 0
	if userIDStr != "" {
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			response.ErrorResponse(w, "用户ID无效", http.StatusBadRequest)
			return
		}
	}

	if userID == 0 {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := db.GetTaskByID(taskID, userID)
		if err != nil {
			if strings.Contains(err.Error(), "任务不存在") {
				response.ErrorResponse(w, "任务不存在", http.StatusNotFound)
				return
			}
			response.ErrorResponse(w, "获取任务失败", http.StatusInternalServerError)
			return
		}
		response.SuccessResponse(w, task, http.StatusOK)

	case http.MethodPatch:
		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
			return
		}

		title, _ := updateData["title"].(string)
		status, _ := updateData["status"].(string)
		description, _ := updateData["description"].(string)
		priority, _ := updateData["priority"].(string)

		dueAtRaw, hasDueAt := updateData["due_at"]
		dueAt := ""
		if hasDueAt {
			dueAt, _ = dueAtRaw.(string)
			if dueAt != "" {
				if _, err := time.Parse(time.RFC3339, dueAt); err != nil {
					if _, err2 := time.Parse("2006-01-02", dueAt); err2 != nil {
						response.ErrorResponse(w, "截止日期格式无效", http.StatusBadRequest)
						return
					}
				}
			}
		}

		if title == "" && status == "" && description == "" && priority == "" && !hasDueAt {
			response.ErrorResponse(w, "没有要更新的字段", http.StatusBadRequest)
			return
		}

		err := db.UpdateTaskFull(taskID, userID, title, description, status, priority, dueAt, hasDueAt)
		if err != nil {
			if strings.Contains(err.Error(), "任务不存在") {
				response.ErrorResponse(w, "任务不存在", http.StatusNotFound)
				return
			}
			response.ErrorResponse(w, "更新任务失败", http.StatusInternalServerError)
			return
		}

		task, _ := db.GetTaskByID(taskID, userID)
		response.SuccessResponse(w, task, http.StatusOK)

	case http.MethodDelete:
		err := db.DeleteTask(taskID, userID)
		if err != nil {
			if strings.Contains(err.Error(), "任务不存在") {
				response.ErrorResponse(w, "任务不存在", http.StatusNotFound)
				return
			}
			response.ErrorResponse(w, "删除任务失败", http.StatusInternalServerError)
			return
		}
		response.SuccessResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)

	default:
		response.ErrorResponse(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}

type syncReq struct {
	LastSyncAt string `json:"last_sync_at"`
	Changes    []struct {
		LocalID string                 `json:"local_id"`
		Op      string                 `json:"op"`
		Payload map[string]interface{} `json:"payload"`
		CV      int                    `json:"client_version"`
	} `json:"changes"`
}

func handleBatchDeleteTasks(w http.ResponseWriter, r *http.Request, wsHub *wsclient.Hub) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req struct {
		TaskIDs []int64 `json:"task_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请�数据", http.StatusBadRequest)
		return
	}

	if len(req.TaskIDs) == 0 {
		response.ErrorResponse(w, "未选择要删除的任务", http.StatusBadRequest)
		return
	}

	// 批量软删除任务
	count, err := db.BatchDeleteTasks(userID, req.TaskIDs)
	if err != nil {
		log.Printf("批量删除失败: %v", err)
		response.ErrorResponse(w, "批量删除失败", http.StatusInternalServerError)
		return
	}

	// 发送通知
	sendNotificationToUser(userID, "tasks_deleted", "任务已删除", fmt.Sprintf("已删除 %d 个任务，30秒内可撤销", count), "normal", wsHub)

	response.SuccessResponse(w, map[string]interface{}{
		"status":              "deleted",
		"count":               count,
		"can_undo":            true,
		"undo_window_seconds": 30,
	}, http.StatusOK)
}

// handleRestoreTask 恢复删除的任务（撤销）

func handleRestoreTask(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	taskID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的任务ID", http.StatusBadRequest)
		return
	}

	userID, _ := strconv.Atoi(userIDStr)

	// 恢复任务
	err = db.RestoreDeletedTask(taskID, userID)
	if err != nil {
		if _, ok := err.(*db.UndoExpiredError); ok {
			response.ErrorResponse(w, "已超过30秒撤销期限", http.StatusGone)
		} else {
			response.ErrorResponse(w, "恢复失败", http.StatusInternalServerError)
		}
		return
	}

	response.SuccessResponse(w, map[string]string{
		"status": "restored",
	}, http.StatusOK)
}

// handleImport 导入任务数据（JSON 或 CSV 格式）

func intelligentMerge(serverTitle, serverDesc, serverStatus string, clientProps map[string]interface{}) (string, string, string) {
	mergedTitle := serverTitle
	mergedDesc := serverDesc
	mergedStatus := serverStatus

	// 标题：优先服务器
	if clientTitle, ok := clientProps["title"].(string); ok && clientTitle != "" {
		if clientTitle != serverTitle {
			mergedTitle = serverTitle
		} else {
			mergedTitle = clientTitle
		}
	}

	// 描述：合并两者的描述
	if clientDesc, ok := clientProps["description"].(string); ok {
		if clientDesc != serverDesc && serverDesc != "" && clientDesc != "" {
			mergedDesc = fmt.Sprintf("[服务器更新] %s\n\n[客户端更新] %s", serverDesc, clientDesc)
		} else if clientDesc != "" {
			mergedDesc = clientDesc
		}
	}

	// 状态：保留较新的状态
	if clientStatus, ok := clientProps["status"].(string); ok {
		if clientStatus != serverStatus {
			mergedStatus = types.PrioritizeStatus(clientStatus, serverStatus)
		}
	}

	return mergedTitle, mergedDesc, mergedStatus
}

// recordConflict 记录冲突到数据库

func recordConflict(localID string, serverID int64, reason string, userID int) {
	options := []types.ConflictResolution{types.ConflictKeepServer, types.ConflictKeepClient, types.ConflictMerge}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		log.Printf("序列化冲突选项错误: %v", err)
		optionsJSON = []byte("[]")
	}
	if err := db.RecordConflict(localID, serverID, reason, string(optionsJSON)); err != nil {
		log.Printf("记录冲突错误: %v", err)
	}
}

// randomString 生成随机字符串（用于local_id冲突处理）
