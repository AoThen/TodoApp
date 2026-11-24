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
	"todoapp/internal/config"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/utils"
)

func toImportString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%f", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return ""
	}
}

// handleBatchDeleteTasks 批量删除任务

func handleImport(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	// 解析 multipart/form-data
	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		response.ErrorResponse(w, "文件过大或格式错误", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.ErrorResponse(w, "未找到文件", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if file == nil {
		response.ErrorResponse(w, "文件不能为空", http.StatusBadRequest)
		return
	}

	// 获取格式
	format := r.FormValue("format")
	if format == "" {
		header.Filename = strings.ToLower(header.Filename)
		if strings.HasSuffix(header.Filename, ".csv") {
			format = "csv"
		} else if strings.HasSuffix(header.Filename, ".json") {
			format = "json"
		} else {
			format = "json"
		}
	}

	var tasks []map[string]interface{}
	var importedCount, skippedCount int

	if format == "json" {
		// 解析JSON
		var jsonTasks []map[string]interface{}
		if err := json.NewDecoder(file).Decode(&jsonTasks); err != nil {
			log.Printf("JSON解析失败: %v", err)
			response.ErrorResponse(w, "JSON解析失败", http.StatusBadRequest)
			return
		}

		for _, jsonTask := range jsonTasks {
			task := map[string]interface{}{
				"local_id":    toImportString(jsonTask["local_id"]),
				"title":       toImportString(jsonTask["title"]),
				"description": toImportString(jsonTask["description"]),
				"status":      toImportString(jsonTask["status"]),
				"priority":    toImportString(jsonTask["priority"]),
				"due_at":      toImportString(jsonTask["due_at"]),
				// sort_order 保留原始数值类型（float64），BatchInsertTasks 内做数值/字符串兼容
				"sort_order":           jsonTask["sort_order"],
				"recurrence_type":      toImportString(jsonTask["recurrence_type"]),
				"recurrence_series_id": toImportString(jsonTask["recurrence_series_id"]),
			}

			if task["title"] != "" {
				tasks = append(tasks, task)
			} else {
				skippedCount++
			}
		}
	} else if format == "csv" {
		// 解析CSV
		reader := utils.NewCSVReader(file)
		csvRecords, err := reader.ReadAll()
		if err != nil {
			log.Printf("CSV解析失败: %v", err)
			response.ErrorResponse(w, "CSV解析失败", http.StatusBadRequest)
			return
		}

		for _, record := range csvRecords {
			title := record["title"]
			if title == "" {
				skippedCount++
				continue
			}

			description := record["description"]
			status := record["status"]
			if status == "" {
				status = "todo"
			}
			priority := record["priority"]
			if priority == "" {
				priority = "medium"
			}

			task := map[string]interface{}{
				"local_id":             fmt.Sprintf("import-%d", time.Now().UnixNano()),
				"title":                title,
				"description":          description,
				"status":               status,
				"priority":             priority,
				"due_at":               record["due_at"],
				"sort_order":           record["sort_order"],
				"recurrence_type":      record["recurrence_type"],
				"recurrence_series_id": record["recurrence_series_id"],
			}
			tasks = append(tasks, task)
		}
	} else {
		response.ErrorResponse(w, "不支持的格式，请使用json或csv", http.StatusBadRequest)
		return
	}

	if len(tasks) == 0 {
		response.ErrorResponse(w, "没有可导入的任务数据", http.StatusBadRequest)
		return
	}

	// 批量插入
	insertedIDs, err := db.BatchInsertTasks(userIDInt, tasks)
	if err != nil {
		log.Printf("批量插入失败: %v", err)
		response.ErrorResponse(w, "导入失败", http.StatusInternalServerError)
		return
	}

	importedCount = len(insertedIDs)

	// 记录导入审计日志
	if err := db.LogExportAction(userIDInt, "import", format, importedCount); err != nil {
		log.Printf("记录导入审计日志错误: %v", err)
	}

	response.SuccessResponse(w, map[string]interface{}{
		"status":       "imported",
		"imported":     importedCount,
		"skipped":      skippedCount,
		"inserted_ids": insertedIDs,
	}, http.StatusOK)
}

// handleExport 导出数据（任务）为 JSON 或 CSV 格式

func handleExport(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户 ID
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	t := r.URL.Query().Get("type")
	if t != "tasks" {
		response.ErrorResponse(w, "不支持的导出类型", http.StatusBadRequest)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment;filename=tasks.json")

		streaming := config.EnvBool("EXPORT_STREAMING_ENABLED", true)
		var buffered []map[string]interface{}
		first := true
		writeJSONTask := func(task map[string]interface{}) error {
			if !first {
				_, _ = w.Write([]byte(",\n"))
			}
			first = false
			return json.NewEncoder(w).Encode(task)
		}

		_, _ = w.Write([]byte("[\n"))
		if err := db.GetTasksStreaming(userIDInt, 100, func(batch []map[string]interface{}) error {
			if !streaming {
				// 非流式模式：全部载入内存后统一写出
				buffered = append(buffered, batch...)
				return nil
			}
			for _, task := range batch {
				if err := writeJSONTask(task); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
			return
		}
		if !streaming {
			for _, task := range buffered {
				if err := writeJSONTask(task); err != nil {
					response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
					return
				}
			}
		}
		_, _ = w.Write([]byte("\n]"))

		if streaming {
			log.Printf("用户 %d 流式导出任务到 JSON 格式", userIDInt)
		} else {
			log.Printf("用户 %d 导出任务到 JSON 格式（非流式，共 %d 条）", userIDInt, len(buffered))
		}
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment;filename=tasks.csv")

		// 写入UTF-8 BOM（Excel兼容）
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

		// 使用CSVStreamer进行流式写入（EXPORT_STREAMING_ENABLED=false 时先缓存再统一写出）
		streaming := config.EnvBool("EXPORT_STREAMING_ENABLED", true)
		var buffered []map[string]interface{}
		normalizeTask := func(task map[string]interface{}) {
			if task["description"] == nil {
				task["description"] = ""
			}
		}
		streamer := utils.NewCSVStreamer(w)
		defer streamer.Close()

		headers := []string{
			"id", "local_id", "server_version", "title", "description",
			"status", "priority", "due_at", "created_at", "updated_at",
			"completed_at", "is_deleted", "last_modified",
			"sort_order", "recurrence_type", "recurrence_series_id",
		}
		if err := streamer.WriteHeader(headers); err != nil {
			response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
			return
		}

		totalExported := 0
		if err := db.GetTasksStreaming(userIDInt, 100, func(batch []map[string]interface{}) error {
			if !streaming {
				buffered = append(buffered, batch...)
				return nil
			}
			for _, task := range batch {
				normalizeTask(task)
				if err := streamer.WriteRow(task); err != nil {
					return err
				}
			}
			totalExported += len(batch)
			return nil
		}); err != nil {
			response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
			return
		}
		if !streaming {
			for _, task := range buffered {
				normalizeTask(task)
				if err := streamer.WriteRow(task); err != nil {
					response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
					return
				}
			}
			totalExported += len(buffered)
		}

		// 记录导出审计日志
		if err := db.LogExportAction(userIDInt, "tasks", format, totalExported); err != nil {
			log.Printf("记录导出审计日志错误: %v", err)
		}

		log.Printf("用户 %d 流式导出了 %d 个任务到 CSV 格式（分批: 100条/批）", userIDInt, totalExported)
		return
	}

	response.ErrorResponse(w, "未知的格式", http.StatusBadRequest)
}

// intelligentMerge 策略：字段级冲突智能合并
