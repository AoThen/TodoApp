package crypto

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"todoapp/internal/response"
)

var (
	encryptionMiddlewareOnce sync.Once
	encryptionMiddleware     *EncryptionMiddleware
)

type EncryptionMiddleware struct {
	cm *CryptoManager
}

func GetEncryptionMiddleware() *EncryptionMiddleware {
	encryptionMiddlewareOnce.Do(func() {
		encryptionMiddleware = &EncryptionMiddleware{
			cm: GetManager(),
		}
	})
	return encryptionMiddleware
}

var noEncryptionPaths = map[string]bool{
	"/api/v1/health":             true,
	"/api/v1/auth/login":         true,
	"/api/v1/auth/refresh":       true,
	"/api/v1/auth/logout":        true,
	"/api/v1/admin/config":       true,
	"/api/v1/admin/users":        true,
	"/api/v1/admin/users/create": true,
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

func (em *EncryptionMiddleware) EncryptRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Encrypted") != "true" {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			response.ErrorResponse(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		decrypted, err := em.cm.Decrypt(string(body))
		if err != nil {
			log.Printf("Decryption failed: %v", err)
			response.ErrorResponse(w, "invalid encrypted data", http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(decrypted))
		next.ServeHTTP(w, r)
	})
}

func (em *EncryptionMiddleware) WrapResponseWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Accept-Encrypted") != "true" {
			next.ServeHTTP(w, r)
			return
		}

		crw := &capturingResponseWriter{
			ResponseWriter: w,
			body:           bytes.NewBuffer(nil),
		}

		next.ServeHTTP(crw, r)

		if crw.body.Len() > 0 {
			encrypted, err := em.cm.EncryptBytes(crw.body.Bytes())
			if err != nil {
				log.Printf("Encryption failed: %v", err)
				response.ErrorResponse(w, "encryption failed", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Encrypted", "true")
			w.Write(encrypted)
		}
	})
}

type capturingResponseWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (crw *capturingResponseWriter) Write(b []byte) (int, error) {
	return crw.body.Write(b)
}

func (em *EncryptionMiddleware) DecryptRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Encrypted") != "true" {
			next.ServeHTTP(w, r)
			return
		}

		if em.ShouldEncrypt(r.URL.Path, r.Method) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				response.ErrorResponse(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			decrypted, err := em.cm.Decrypt(string(body))
			if err != nil {
				log.Printf("Decryption failed: %v", err)
				response.ErrorResponse(w, "invalid encrypted data", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(strings.NewReader(decrypted))
		}

		next.ServeHTTP(w, r)
	})
}

func (em *EncryptionMiddleware) EncryptResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Accept-Encrypted") != "true" {
			next.ServeHTTP(w, r)
			return
		}

		if !em.ShouldEncrypt(r.URL.Path, r.Method) && r.URL.Path != "/api/v1/sync" {
			next.ServeHTTP(w, r)
			return
		}

		crw := &capturingResponseWriter{
			ResponseWriter: w,
			body:           bytes.NewBuffer(nil),
		}

		next.ServeHTTP(crw, r)

		if crw.body.Len() > 0 {
			encrypted, err := em.cm.EncryptBytes(crw.body.Bytes())
			if err != nil {
				log.Printf("Encryption failed: %v", err)
				response.ErrorResponse(w, "encryption failed", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Encrypted", "true")
			w.Write(encrypted)
		}
	})
}

type encryptionHandler struct {
	next        http.Handler
	em          *EncryptionMiddleware
	encryptReq  bool
	encryptResp bool
}

func NewEncryptionHandler(next http.Handler, encryptReq, encryptResp bool) http.Handler {
	return &encryptionHandler{
		next:        next,
		em:          GetEncryptionMiddleware(),
		encryptReq:  encryptReq,
		encryptResp: encryptResp,
	}
}

func (eh *encryptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if eh.encryptReq && r.Header.Get("X-Encrypted") == "true" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			response.ErrorResponse(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		decrypted, err := eh.em.cm.Decrypt(string(body))
		if err != nil {
			log.Printf("Decryption failed: %v", err)
			response.ErrorResponse(w, "invalid encrypted data", http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(decrypted))
	}

	if eh.encryptResp && r.Header.Get("X-Accept-Encrypted") == "true" {
		crw := &capturingResponseWriter{
			ResponseWriter: w,
			body:           bytes.NewBuffer(nil),
		}

		eh.next.ServeHTTP(crw, r)

		if crw.body.Len() > 0 {
			encrypted, err := eh.em.cm.EncryptBytes(crw.body.Bytes())
			if err != nil {
				log.Printf("Encryption failed: %v", err)
				response.ErrorResponse(w, "encryption failed", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Encrypted", "true")
			w.Write(encrypted)
		}
	} else {
		eh.next.ServeHTTP(w, r)
	}
}

func GetRequestEncryptionStatus(r *http.Request) bool {
	return r.Header.Get("X-Encrypted") == "true"
}

func GetResponseEncryptionStatus(r *http.Request) bool {
	return r.Header.Get("X-Accept-Encrypted") == "true"
}

func ShouldEncryptPath(path, method string) bool {
	if noEncryptionPaths[path] {
		return false
	}

	if path == "/api/v1/sync" {
		return method == http.MethodPost
	}

	if strings.HasPrefix(path, "/api/v1/tasks") {
		return method != http.MethodGet && method != http.MethodDelete
	}

	return false
}
