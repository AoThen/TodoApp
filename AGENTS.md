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
- Data export (JSON/CSV) with streaming support
- Login rate limiting, account lockout
- JWT authentication (access + refresh tokens)

**Web**: Task list, notifications, admin panel, device pairing via QR code
**Android**: Room database, WorkManager sync, QR scanner, local notifications

## Build, Lint, and Test Commands

### Backend (Go)
```bash
go run main.go                              # Run dev server
CGO_ENABLED=1 go build -o todoapp-server .  # Production build

# 测试（auth 包 init() 需要 JWT_SECRET 环境变量，至少 32 字符）
JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test ./...  # All tests
JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -v ./internal/db  # Specific package
JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -v ./internal/db -run TestUpdateTaskFullOwnership  # Single test
JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -v -run ^TestFuncName$ ./...  # Run specific test function
JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test -v -count=1 -race ./...  # Run with race detection
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
./gradlew lint                              # Android lint
./gradlew check                             # Run all checks
```

### Docker
```bash
make build          # Build Docker image
make run            # Start containers
make stop           # Stop containers
make restart        # Restart containers
make logs           # View container logs
make logs-tail      # View last 100 log lines
make status         # Container status and resource usage
make shell          # Enter container shell
make health         # Health check endpoint
make clean          # Cleanup containers and images
make db-backup      # Backup database to backup/
make db-restore     # Restore database from backup
make rebuild        # Rebuild and restart
make monitor        # Real-time resource monitoring
make deploy         # Production deployment
make dev            # Development environment
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

## Backend Best Practices (Go)
- Implement conflict resolution with version control (client_version/server_version)
- Follow defensive programming: validate inputs, handle all possible states
- Use context for request-scoped values and cancellation
- Implement proper rate limiting and security middleware
- Use structured logging for debugging and monitoring

## Frontend Best Practices (TypeScript/React)
- Use memoization (React.memo, useMemo, useCallback) to optimize performance
- Implement proper error boundaries to handle unexpected errors
- Handle loading and error states in UI
- Implement retry logic for failing API calls
- Use TypeScript for type safety and better refactoring
- Follow MVVM pattern with services for API communication

## Android Best Practices (Kotlin)
- Use Hilt for dependency injection
- Follow MVVM architecture pattern
- Use View Binding for view references
- Implement error handling and network state awareness
- Use WorkManager for background sync operations
- Leverage coroutines for asynchronous operations

## Common Patterns

### API Response (Go)
```go
response.SuccessResponse(w, data, http.StatusOK)
response.ErrorResponse(w, "错误消息", http.StatusBadRequest)
```

### Delta Sync Flow
1. Store local changes in delta queue
2. Sync periodically (`web/src/services/syncManager.ts`)
3. Send deltas/receive changes
4. Apply and clear confirmed
5. Track conflicts

### File Structure
```
/: main.go, Makefile, docker-compose.yml, Dockerfile
internal/: auth/, cache/, config/, crypto/, db/, response/, types/, utils/, validator/, websocket/
web/: src/{components/,services/,utils/,App.tsx}
android/: src/main/java/com/todoapp/{data/,ui/}/
```

## Security Requirements
- User-facing errors in Chinese (中文)
- Never log/expose JWT_SECRET, passwords, or sensitive data
- Use bcrypt for password hashing
- Validate all inputs with `validator` package
- Sanitize to prevent XSS/SQL injection
- WebSocket connections use encryption (`internal/websocket/encryption.go`)
- Production environment enforces encrypted WebSocket connections

## Testing Conventions
- Go: `*_test.go` files alongside source, use `testing` package
- React: `*.test.tsx` or `*.test.ts` files, Jest/React Testing Library
- Android: `src/test/` (unit), `src/androidTest/` (instrumentation)

## API Endpoints
All endpoints under `/api/v1/`:
- `/api/v1/auth/` - Authentication (login, refresh, logout)
- `/api/v1/tasks/` - Task CRUD with pagination
- `/api/v1/tasks/batch` - Batch operations (DELETE)
- `/api/v1/admin/` - Admin ops (users, logs, config)
- `/api/v1/notifications/` - Notification management
- `/api/v1/export` - Data export (JSON/CSV)
- `/api/v1/sync` - Delta sync
- `/ws` - WebSocket
- `/api/v1/health` - Health check

## Database
- SQLite for local storage
- Path: `/app/data/todoapp.db` in Docker
- Tables: users, tasks, notifications, delta_queue, devices, login_logs, tokens, conflicts, deleted_tasks

## Environment Variables

### Required
- `JWT_SECRET` - JWT signing key (minimum 32 characters)
- `INITIAL_ADMIN_EMAIL/PASSWORD` - Required for first startup

### Security
- `ENCRYPTION_KEY` - AES-256-GCM encryption key (32-byte hex)
- `RATE_LIMIT_PER_MINUTE` - Per-IP request rate limit (default: 600)
- `MAX_LOGIN_ATTEMPTS` - Max failed login attempts (default: 5)
- `LOGIN_ATTEMPT_WINDOW` - Failed attempt window (default: 15m)
- `LOCKOUT_DURATION` - Account lockout duration (default: 30m)

### Token Configuration
- `ACCESS_TOKEN_DURATION` - Access token validity (default: 15m)
- `REFRESH_TOKEN_DURATION` - Refresh token validity (default: 168h/7 days)

### Feature Flags
- `ENVIRONMENT` - development | production
- `SERVER_PORT` - Server port (default: 8080)
- `ENFORCE_WS_ENCRYPTION` - Force encrypted WebSocket (default: true in production)
- `AUTO_REDIRECT_HTTPS` - HTTPS redirect (production)
- `EXPORT_STREAMING_ENABLED` - Enable streaming export (default: true)
- `NOTIFICATION_RETENTION_DAYS` - Notification retention (default: 30)

## Docker Configuration
- Multi-stage build (frontend-builder, backend-builder, runtime)
- Runs as non-root user (appuser)
- Health check enabled
- Optional Nginx reverse proxy (profile: with-nginx)
- Resource limits: 1 CPU, 512MB memory

## Development Workflow
1. Always check for existing issues before implementing new features
2. Follow existing code patterns and conventions in the codebase
3. Write tests for new functionality
4. Run all tests before submitting changes
5. Document API changes in corresponding documentation
6. Update README if new features affect user experience

## Debugging Tips
- Use `log.Printf` strategically for debugging backend issues
- Check browser developer tools for web frontend issues
- Use Android Studio's logcat for Android debugging
- Monitor sync flow carefully when implementing delta sync changes
- Test offline/online transitions thoroughly
- Use `make logs` or `make logs-tail` for container debugging

## Performance Considerations
- Implement proper pagination for large datasets
- Use streaming for large data exports (CSV/JSON)
- Optimize database queries with appropriate indexes
- Minimize WebSocket message size
- Implement proper caching strategies
- Use lazy loading for long lists
- Use batch operations for bulk updates (`internal/db/batch_operations_optimized.go`)

## Common Pitfalls to Avoid
- Never store sensitive data (JWT secrets, passwords) in source code
- Avoid N+1 query problems when fetching related data
- Don't forget to handle race conditions in concurrent operations
- Always validate user input on both client and server
- Don't expose internal error messages to end users
- Avoid hardcoding values that should be configurable
- Ensure proper cleanup of resources (database connections, file handles)
- Use transactions for operations that must be atomic