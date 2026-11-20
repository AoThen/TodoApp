package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccessResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	SuccessResponse(rec, map[string]string{"id": "1"}, http.StatusOK)

	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v", err)
	}
	if !body.Success || body.Data["id"] != "1" {
		t.Errorf("响应体不符合契约: %v", body)
	}
}

func TestErrorResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	ErrorResponse(rec, "用户不存在", http.StatusNotFound)

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码 = %d, want 404", rec.Code)
	}
	var body struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v", err)
	}
	if body.Success || body.Error != "用户不存在" {
		t.Errorf("响应体不符合契约: %v", body)
	}
}

func TestValidationErrorResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	ValidationErrorResponse(rec, map[string]string{"email": "邮箱格式不正确"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("状态码 = %d, want 400", rec.Code)
	}
	var body struct {
		Success bool              `json:"success"`
		Error   string            `json:"error"`
		Fields  map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v", err)
	}
	if body.Success || body.Fields["email"] != "邮箱格式不正确" {
		t.Errorf("响应体不符合契约: %v", body)
	}
}
