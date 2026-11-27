package crypto

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"

	"todoapp/internal/response"
)

// KeyResolver 依据请求解析出该设备对应的加密管理器。
// 未找到/未授权时应返回错误，中间件将以明文错误响应（fail-closed）。
type KeyResolver func(r *http.Request) (*CryptoManager, error)

// EncryptionMiddleware 按请求动态选择设备密钥的加解密中间件
type EncryptionMiddleware struct {
	resolve          KeyResolver
	requireEncrypted bool
}

// NewEncryptionMiddleware 创建中间件。
// requireEncrypted 为 true 时，对加密路径拒绝未携带 X-Encrypted 的明文请求。
func NewEncryptionMiddleware(resolve KeyResolver, requireEncrypted bool) *EncryptionMiddleware {
	return &EncryptionMiddleware{resolve: resolve, requireEncrypted: requireEncrypted}
}

var noEncryptionPaths = map[string]bool{
	"/api/v1/health":             true,
	"/api/v1/auth/login":         true,
	"/api/v1/auth/refresh":       true,
	"/api/v1/auth/logout":        true,
	"/api/v1/admin/config":       true,
	"/api/v1/admin/users":        true,
	"/api/v1/admin/users/create": true,
	"/ws":                        true,
}

func (em *EncryptionMiddleware) ShouldEncrypt(path string, method string) bool {
	if noEncryptionPaths[path] {
		return false
	}
	if path == "/api/v1/sync" || strings.HasPrefix(path, "/api/v1/tasks") {
		return method != http.MethodGet
	}
	if strings.HasPrefix(path, "/api/v1/admin/") {
		return false
	}
	return false
}

// enforce 在开启强制加密时拒绝加密路径携带 body 的明文请求。
// 无 body 的请求（如 DELETE /tasks/{id}）没有可解密内容，不允许强制加密。
func (em *EncryptionMiddleware) enforce(path, method string, w http.ResponseWriter, encrypted, hasBody bool) bool {
	if em.requireEncrypted && hasBody && em.ShouldEncrypt(path, method) && !encrypted {
		response.ErrorResponse(w, "该接口要求加密传输", http.StatusBadRequest)
		return false
	}
	return true
}

func (em *EncryptionMiddleware) DecryptRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encrypted := r.Header.Get("X-Encrypted") == "true"
		hasBody := r.ContentLength != 0
		if !em.enforce(r.URL.Path, r.Method, w, encrypted, hasBody) {
			return
		}

		if !encrypted || !em.ShouldEncrypt(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			response.ErrorResponse(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		cm, err := em.resolve(r)
		if err != nil {
			log.Printf("解析设备加密密钥失败: %v", err)
			response.ErrorResponse(w, "设备加密密钥不可用", http.StatusBadRequest)
			return
		}

		decrypted, err := cm.Decrypt(string(body))
		if err != nil {
			log.Printf("Decryption failed: %v", err)
			response.ErrorResponse(w, "invalid encrypted data", http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(decrypted))
		next.ServeHTTP(w, r)
	})
}

func (em *EncryptionMiddleware) EncryptResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Accept-Encrypted") != "true" || !em.ShouldEncrypt(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		cm, err := em.resolve(r)
		if err != nil {
			log.Printf("解析设备加密密钥失败: %v", err)
			response.ErrorResponse(w, "设备加密密钥不可用", http.StatusBadRequest)
			return
		}

		crw := &capturingResponseWriter{
			ResponseWriter: w,
			body:           bytes.NewBuffer(nil),
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(crw, r)

		status := crw.statusCode
		if status == 0 {
			status = http.StatusOK
		}

		if crw.body.Len() == 0 {
			w.WriteHeader(status)
			return
		}

		encrypted, err := cm.EncryptBytes(crw.body.Bytes())
		if err != nil {
			log.Printf("Encryption failed: %v", err)
			response.ErrorResponse(w, "encryption failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Encrypted", "true")
		w.WriteHeader(status)
		_, _ = w.Write(encrypted)
	})
}

type capturingResponseWriter struct {
	http.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (crw *capturingResponseWriter) WriteHeader(code int) {
	if crw.wroteHeader {
		return
	}
	crw.statusCode = code
	crw.wroteHeader = true
}

func (crw *capturingResponseWriter) Write(b []byte) (int, error) {
	if !crw.wroteHeader {
		crw.WriteHeader(http.StatusOK)
	}
	return crw.body.Write(b)
}
