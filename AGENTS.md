# AGENTS.md - Developer Guidelines for TodoApp

Full-stack todo app: Go backend, React/TypeScript frontend, Android native app.

## Build, Lint, and Test Commands

### Backend (Go)
```bash
# Run dev server: go run main.go
# Build: CGO_ENABLED=1 go build -o todoapp-server .

# Run all tests: go test ./...
# Specific package: go test -v ./internal/db
# Specific test: go test -v ./internal/db -run TestInitDB

# Docker: make build | make run | make test | make logs | make clean
```

### Frontend (React/TypeScript)
```bash
cd web
npm install
npm start                     # dev server
npm run build                 # production build
npm test                      # run tests
npm test -- --watch           # watch mode
npm test -- App.test.tsx      # single file
npm test -- --testNamePattern="handleLogin"  # by name
```

### Android (Kotlin)
```bash
cd android
./gradlew assembleDebug       # debug APK
./gradlew assembleRelease     # release APK
./gradlew test                # unit tests
./gradlew connectedAndroidTest    # instrumentation
./gradlew test --tests "*.DeltaSyncWorkerTest"
./gradlew clean               # clean build
```

## Code Style Guidelines

### Go (Backend)
**Imports**: stdlib, third-party, internal (blank line between groups)
```go
import (
    "context"
    "net/http"

    "github.com/gorilla/mux"

    "todoapp/internal/db"
)
```

**Naming**: Exported = PascalCase, unexported = camelCase, constants = UPPER_SNAKE_CASE, interfaces = -er suffix. Package names = lowercase single word.

**Types**: Use explicit types, avoid `any`. Strongly typed structs with JSON tags.

**Error Handling**: Return explicit errors. User-facing errors in Chinese.
```go
if err != nil {
    return fmt.Errorf("数据库初始化失败: %w", err)
}
response.ErrorResponse(w, "用户不存在", http.StatusNotFound)
```

**Concurrency**: Use `sync.RWMutex` for shared maps, defer unlocks.

### TypeScript/React (Frontend)
**Imports**: React/third-party first, then local.

**Naming**: Components = PascalCase, functions/variables = camelCase, types/interfaces = PascalCase, constants = UPPER_SNAKE_CASE.

**Components**: Functional + hooks only (no class components).

**Async**: Always use async/await with try-catch error handling.

**Types**: Use interfaces/types for all data structures, no `any`.

**Services**: Class-based singletons.

### Kotlin (Android)
**Imports**: Android/Kotlin stdlib, third-party, local (blank line between).

**Naming**: Classes/interfaces = PascalCase, functions/properties = camelCase, constants = UPPER_SNAKE_CASE, objects = PascalCase.

**Data Classes**: Use `data class` with Room annotations.

**Room DAOs**: Interface with suspend functions.

**Coroutines**: Use `suspend` functions, `withContext(Dispatchers.IO)` for threading.

**Singletons**: Use `object` with lazy initialization.

**Nullability**: Explicit nullable types (`String?`, `Task?`).

## Common Patterns

**API Response (Go)**: `response.SuccessResponse(w, data, http.StatusOK)` / `response.ErrorResponse(w, "错误消息", http.StatusBadRequest)`

**Delta Sync**: 1) Store local changes in delta queue, 2) Sync periodically, 3) Send deltas/receive changes, 4) Apply and clear confirmed, 5) Track conflicts.

## Security Requirements
- User-facing errors in Chinese
- Never log/expose JWT_SECRET, passwords, or sensitive data
- Use bcrypt for password hashing
- Validate all inputs with `validator` package
- Sanitize to prevent XSS/SQL injection

## Testing Conventions
- Go: `*_test.go` files alongside source, use `testing` package
- React: `*.test.tsx` or `*.test.ts` files, Jest/React Testing Library
- Android: `src/test/` (unit), `src/androidTest/` (instrumentation)
