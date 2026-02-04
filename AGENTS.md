# AGENTS.md - Developer Guidelines for TodoApp

Full-stack todo app with cross-platform sync: Go backend, React/TypeScript web, Android native app.

## Project Overview

TodoApp is a feature-rich task management application with offline-first architecture and real-time synchronization across devices.

**Core Features:**
- Task CRUD with status (todo/in_progress/done/archived) and priority (low/medium/high)
- Delta sync for conflict-free offline/online transitions
- Real-time WebSocket notifications
- End-to-end encryption (AES-256-GCM) for device pairing
- Device pairing via QR code
- Admin panel for user management and system logs
- Data export (JSON/CSV)
- Login rate limiting, account lockout
- JWT authentication (access + refresh tokens)

**Web**: Task list, notifications, admin panel, device pairing via QR code
**Android**: Room database, WorkManager sync, QR scanner, local notifications

## Build, Lint, and Test Commands

### Backend (Go)
```bash
go run main.go                              # Run dev server
CGO_ENABLED=1 go build -o todoapp-server .  # Production build

go test ./...                               # All tests
go test -v ./internal/db                    # Specific package
go test -v ./internal/db -run TestInitDB    # Single test

make build | make run | make test | make logs | make clean
```

### Frontend (React/TypeScript)
```bash
cd web
npm install && npm start                    # Dev server
npm run build                               # Production build
npm test                                    # Run tests
npm test -- App.test.tsx                    # Single file
```

### Android (Kotlin)
```bash
cd android
./gradlew assembleDebug                     # Debug APK
./gradlew assembleRelease                   # Release APK
./gradlew test                              # Unit tests
./gradlew connectedAndroidTest              # Instrumentation tests
./gradlew test --tests "*.DeltaSyncWorkerTest"  # Specific test
./gradlew clean                             # Clean build
```

### Database
```bash
make db-shell                               # SQLite shell
make db-backup                              # Backup to backup/
make db-restore                             # Restore from backup
```

## Code Style Guidelines

### Go (Backend)
**Imports**: stdlib, third-party, internal (blank line between groups). Package names = lowercase single word.
```go
import (
    "context"
    "net/http"
    "github.com/gorilla/mux"
    "todoapp/internal/db"
)
```

**Naming**: Exported = PascalCase, unexported = camelCase, constants = UPPER_SNAKE_CASE, interfaces = -er suffix.

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
**Imports**: React/third-party first, then local, then relative paths.

**Naming**: Components = PascalCase, functions/variables = camelCase, types/interfaces = PascalCase, constants = UPPER_SNAKE_CASE.
**Components**: Functional + hooks only (no class components).
**Async**: Always use async/await with try-catch error handling.

**Types**: Use interfaces/types for all data structures, avoid `any`.

**Services**: Class-based singletons (web/src/services/*.ts).
### Kotlin (Android)
**Imports**: Android/Kotlin stdlib, third-party, local (blank line between).

**Naming**: Classes/interfaces = PascalCase, functions/properties = camelCase, constants = UPPER_SNAKE_CASE, objects = PascalCase.

**Data Classes**: Use `data class` with Room annotations. DAOs use suspend functions.

**Coroutines**: Use `suspend` functions, `withContext(Dispatchers.IO)` for threading. Explicit nullable types (`String?`, `Task?`).

## Common Patterns

### API Response (Go)
```go
response.SuccessResponse(w, data, http.StatusOK)
response.ErrorResponse(w, "错误消息", http.StatusBadRequest)
```

### Delta Sync Flow
1. Store local changes in delta queue 2. Sync periodically (web/src/services/syncManager.ts)
3. Send deltas/receive changes 4. Apply and clear confirmed 5. Track conflicts

### File Structure
```
backend/: main.go, internal/{auth,db,response,validator,crypto,websocket}/
web/: src/{components/,services/,utils/,App.tsx}/
android/: src/main/java/com/todoapp/{data/,ui/}/
```

## Security Requirements
- User-facing errors in Chinese (中文)
- Never log/expose JWT_SECRET, passwords, or sensitive data
- Use bcrypt for password hashing
- Validate all inputs with `validator` package
- Sanitize to prevent XSS/SQL injection
- WebSocket connections use encryption (internal/websocket/encryption.go)

## Testing Conventions
- Go: `*_test.go` files alongside source, use `testing` package
- React: `*.test.tsx` or `*.test.ts` files, Jest/React Testing Library
- Android: `src/test/` (unit), `src/androidTest/` (instrumentation)

## API Endpoints
All endpoints under `/api/v1/`:
- `/api/v1/auth/` - Authentication (login, refresh, logout)
- `/api/v1/tasks/` - Task CRUD with pagination
- `/api/v1/admin/` - Admin ops (users, logs, config)
- `/api/v1/notifications/` - Notification management
- `/api/v1/export` - Data export (JSON/CSV)
- `/api/v1/sync` - Delta sync, `/ws` - WebSocket, `/api/v1/health` - Health check

## Database
- SQLite for local storage
- Path: `/app/data/todoapp.db` in Docker
- Tables: users, tasks, notifications, delta_queue, devices, login_logs, tokens, conflicts
