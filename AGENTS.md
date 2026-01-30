# AGENTS.md - Developer Guidelines for TodoApp

This guide is for agentic coding assistants working on the TodoApp repository.

## Project Overview
Full-stack todo application with Go backend, React/TypeScript frontend, and Android native app.
- Backend: Go 1.20+, SQLite (WAL mode), Gorilla Mux, JWT auth
- Frontend: React 18, TypeScript, Axios, IndexedDB (idb)
- Android: Kotlin, Room, WorkManager, Retrofit, OkHttp

---

## Build, Lint, and Test Commands

### Backend (Go)
```bash
# Run development server
go run main.go

# Build executable
CGO_ENABLED=1 go build -o todoapp-server .

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/db
go test -v ./internal/auth

# Run specific test function
go test -v ./internal/db -run TestInitDB

# Docker commands (via Makefile)
make build      # Build Docker image
make run        # Start containers
make test       # Run tests in Docker
make logs       # View logs
make clean      # Clean containers/images
```

### Frontend (React/TypeScript)
```bash
cd web

# Install dependencies
npm install

# Start development server
npm start

# Build for production
npm run build

# Run tests
npm test

# Run tests in watch mode
npm test -- --watch

# Run single test file
npm test -- App.test.tsx

# Run tests matching pattern
npm test -- --testNamePattern="handleLogin"
```

### Android (Kotlin)
```bash
cd android

# Build debug APK
./gradlew assembleDebug

# Build release APK
./gradlew assembleRelease

# Run unit tests
./gradlew test

# Run instrumentation tests
./gradlew connectedAndroidTest

# Run specific test class
./gradlew test --tests "*.DeltaSyncWorkerTest"

# Clean build
./gradlew clean
```

---

## Code Style Guidelines

### Go (Backend)
**Imports**: Group in order: stdlib, third-party, internal packages
```go
import (
    "context"
    "net/http"                    // stdlib

    "github.com/gorilla/mux"      // third-party

    "todoapp/internal/db"         // internal
)
```

**Naming**:
- Exported: PascalCase (`UserService`, `InitDB`)
- Unexported: camelCase (`handleLogin`, `syncInProgress`)
- Constants: UPPER_SNAKE_CASE (`MAX_LOGIN_ATTEMPTS`)
- Interfaces: PascalCase with -er suffix if appropriate (`AuthHandler`)
- Package names: lowercase, single word (`auth`, `db`, `validator`)

**Formatting**: Use `gofmt` or `go fmt` - standard Go formatting

**Types**: Use explicit types, avoid `any`. Prefer strongly typed structs
```go
type Task struct {
    ID          int       `json:"id"`
    UserID      int       `json:"user_id"`
    Title       string    `json:"title"`
    Status      string    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
}
```

**Error Handling**: Return explicit errors, use Chinese error messages for user-facing errors
```go
if err != nil {
    return fmt.Errorf("数据库初始化失败: %w", err)
}
response.ErrorResponse(w, "用户不存在", http.StatusNotFound)
```

**Concurrency**: Use `sync.RWMutex` for shared maps, defer unlocks
```go
attemptsMutex.Lock()
defer attemptsMutex.Unlock()
```

**Constants**: Define package-level constants for magic numbers
```go
const (
    maxLoginAttempts = 5
    loginAttemptWindow = 15 * time.Minute
    accessTokenDuration = 15 * time.Minute
)
```

### TypeScript/React (Frontend)
**Imports**: Group: React/third-party, then local (services, utils, components)
```typescript
import React, { useState, useEffect } from 'react';
import { apiService } from './services/api';
import { debounce } from './utils/debounce';
```

**Naming**:
- Components: PascalCase (`App`, `TaskItem`)
- Functions/variables: camelCase (`handleLogin`, `syncInProgress`)
- Types/Interfaces: PascalCase (`Task`, `DeltaChange`)
- Constants: UPPER_SNAKE_CASE (`REQUEST_TIMEOUT`)

**Components**: Functional components with hooks only (no class components)
```typescript
const App: React.FC = () => {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    loadTasks();
  }, []);

  const handleAddTask = async () => { ... };

  return <div>{/* JSX */}</div>;
};
```

**Async/Await**: Always use async/await over Promises, handle errors with try-catch
```typescript
try {
  const response = await apiService.login(email, password);
  setAuthToken(response.access_token);
} catch (error) {
  console.error('Login failed:', error);
  alert('Login failed. Please check your credentials.');
}
```

**Types**: Use TypeScript interfaces/types for all data structures, no `any`
```typescript
interface Task {
  local_id: string;
  server_id?: number;
  title: string;
  status: 'todo' | 'done';
  created_at: string;
}
```

**Services**: Use class-based services with singletons
```typescript
class ApiService {
  private client: AxiosInstance;
  
  constructor() { ... }
  
  async login(email: string, password: string): Promise<{access_token: string}> { ... }
}

export const apiService = new ApiService();
```

**Event Handlers**: Explicitly type event parameters
```typescript
const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
  e.preventDefault();
  const email = (e.target as HTMLFormElement).email.value;
};
```

### Kotlin (Android)
**Imports**: Group: Android/Kotlin stdlib, third-party, local
```kotlin
import android.content.Context
import androidx.room.*
import com.todoapp.data.local.Task
```

**Naming**:
- Classes/Interfaces: PascalCase (`Task`, `AppDatabase`, `TaskDao`)
- Functions/properties: camelCase (`getTaskByLocalId`, `isDeleted`)
- Constants: UPPER_SNAKE_CASE (`WORK_NAME`, `BASE_URL`)
- Objects: PascalCase for singletons (`RetrofitClient`)

**Data Classes**: Use `data class` with Room annotations for entities
```kotlin
@Entity(tableName = "tasks")
data class Task(
    @PrimaryKey(autoGenerate = true)
    val id: Long = 0,
    val localId: String,
    val title: String,
    val isDeleted: Boolean = false
)
```

**Room DAOs**: Interface with suspend functions for async operations
```kotlin
@Dao
interface TaskDao {
    @Query("SELECT * FROM tasks WHERE isDeleted = 0")
    suspend fun getAllTasks(): List<Task>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertTask(task: Task): Long
}
```

**Coroutines**: Use `suspend` functions, `withContext` for threading
```kotlin
override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
    try {
        val tasks = database.taskDao().getAllTasks()
        Result.success()
    } catch (e: Exception) {
        Result.failure()
    }
}
```

**Singletons**: Use `object` with lazy initialization and thread-safety
```kotlin
object RetrofitClient {
    @Volatile private var apiService: ApiService? = null

    fun getApiService(context: Context): ApiService {
        return apiService ?: synchronized(this) {
            apiService ?: createApiService(context).also { apiService = it }
        }
    }
}
```

**Nullability**: Use nullable types explicitly (`String?`, `Task?`), avoid platform types
```kotlin
val serverId: Long? = null
val task: Task? = database.taskDao().getTaskById(id)
```

---

## Common Patterns

### API Response Structure (Go)
```go
response.SuccessResponse(w, data, http.StatusOK)
response.ErrorResponse(w, "错误消息", http.StatusBadRequest)
```

### JWT Token Management (Frontend)
```typescript
// Store in localStorage with expiry
localStorage.setItem('access_token', token);
localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000));

// Interceptor adds Authorization header
config.headers.Authorization = `Bearer ${token}`;
```

### Delta Sync Pattern
1. Store local changes in delta queue (insert/update/delete ops)
2. Sync periodically (frontend: setInterval, Android: WorkManager)
3. Send pending deltas to server, receive server changes
4. Apply server changes locally, clear confirmed deltas
5. Track conflicts for manual resolution

### Database Connection (Go)
```go
DB.SetMaxOpenConns(25)
DB.SetMaxIdleConns(5)
DB.SetConnMaxLifetime(5 * time.Minute)
DB.Exec("PRAGMA journal_mode = WAL;")
```

---

## Testing Notes
- No existing test files found in this repository
- When adding tests, follow conventions:
  - Go: Create `*_test.go` files alongside source, use `testing` package
  - React: Create `*.test.tsx` or `*.test.ts` files, use Jest/React Testing Library
  - Android: Create test files in `src/test/` (unit) and `src/androidTest/` (instrumentation)

---

## Security Requirements
- All user-facing errors in Chinese
- Never log or expose JWT_SECRET, passwords, or sensitive data
- Use bcrypt for password hashing
- Validate all inputs using `validator` package (Go)
- Sanitize inputs to prevent XSS and SQL injection

---

## Admin Management

### User Roles
- **admin**: Full access to admin panel, user management, logs, and system configuration
- **user**: Regular user access to tasks only

### Environment Variables for Initial Admin
```bash
INITIAL_ADMIN_EMAIL=admin@example.com    # Default admin email (created if DB is empty)
INITIAL_ADMIN_PASSWORD=Admin123!          # Default admin password
```

### Admin API Endpoints
All admin endpoints require JWT authentication AND admin role (`role = 'admin'`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/users` | List all users |
| POST | `/api/v1/admin/users` | Create new user |
| PATCH | `/api/v1/admin/users/{id}` | Update user (email, role) |
| POST | `/api/v1/admin/users/{id}/password` | Reset user password |
| POST | `/api/v1/admin/users/{id}/lock` | Lock user account |
| POST | `/api/v1/admin/users/{id}/unlock` | Unlock user account |
| DELETE | `/api/v1/admin/users/{id}` | Delete user |
| GET | `/api/v1/admin/logs/login` | Get login logs |
| GET | `/api/v1/admin/logs/actions` | Get admin action logs |
| GET | `/api/v1/admin/config` | Get system config |
| PUT | `/api/v1/admin/config` | Update system config |

### Admin Middleware
```go
// authMiddleware validates JWT tokens
// adminMiddleware checks if user has admin role
admin := router.PathPrefix("/api/v1/admin").Subrouter()
admin.Use(authMiddleware)
admin.Use(adminMiddleware)
```

### Database Tables for Admin
- `users`: Added `role TEXT DEFAULT 'user'` column
- `admin_logs`: Records all admin actions for audit
- `system_config`: Stores system configuration key-value pairs

### Frontend Admin Components
- `components/admin/AdminPanel.tsx`: Main admin panel container
- `components/admin/UserManagement.tsx`: User CRUD operations
- `components/admin/SystemLogs.tsx`: Login and action logs
- `components/admin/SystemConfig.tsx`: System configuration
- `services/admin.ts`: Admin API client service
