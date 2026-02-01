# Android端集成指南

## 快速开始

### 步骤1：配置Android SDK

```bash
# 配置ANDROID_HOME环境变量
export ANDROID_HOME=/path/to/android/sdk

# 或在项目根目录创建local.properties
echo "sdk.dir=/path/to/android/sdk" > android/local.properties
```

### 步骤2：同步Gradle

```bash
cd android
./gradlew sync
```

### 步骤3：在MainActivity中通知服务

```kotlin
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import com.todoapp.data.notify.NotificationManager
import com.todoapp.data.notify.NotificationWebSocket
import kotlinx.coroutines.*

class MainActivity : AppCompatActivity() {
    private lateinit var notificationManager: NotificationManager
    private lateinit var notificationWebSocket: NotificationWebSocket
    private val scope = CoroutineScope(Dispatchers.Main + SupervisorJob())

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        // 初始化通知管理器
        val userId = getCurrentUserId() // 从你的用户系统获取
        notificationManager = NotificationManager(this, userId)

        // 连接WebSocket
        val token = getCurrentToken() // 从你的认证系统获取
        notificationWebSocket = NotificationWebSocket(this, token, true)
        notificationWebSocket.connect()

        // 注册消息处理器
        notificationWebSocket.onMessage("notification") { message ->
            handleNotificationMessage(message)
        }
    }

    private fun getCurrentUserId(): Int {
        // 从SharedPreferences或其他存储中获取用户ID
        return getSharedPreferences("user_prefs", MODE_PRIVATE)
            .getInt("user_id", 0)
    }

    private fun getCurrentToken(): String {
        // 从SharedPreferences或其他存储中获取token
        return getSharedPreferences("auth_prefs", MODE_PRIVATE)
            .getString("access_token", "") ?: ""
    }

    private fun handleNotificationMessage(message: JSONObject) {
        scope.launch {
            val title = message.optString("title", "新通知")
            val content = message.optString("content", "")

            // 创建本地通知
            notificationManager.createNotification(
                type = message.optString("type", "system"),
                title = title,
                content = content,
                priority = message.optString("priority", "normal")
            )

            // 显示Toast
            runOnUiThread {
                Toast.makeText(this@MainActivity, title, Toast.LENGTH_SHORT).show()
            }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        notificationWebSocket.disconnect()
        scope.coroutineContext.cancel()
        notificationManager.cleanup()
    }
}
```

### 步骤4：添加通知入口

在你的主界面添加通知访问按钮：

```xml
<!-- activity_main.xml -->
<LinearLayout
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:orientation="horizontal">

    <!-- 其他按钮 -->
    <Button
        android:id="@+id/btnTasks"
        android:layout_width="0dp"
        android:layout_weight="1"
        android:layout_height="wrap_content"
        android:text="任务" />

    <!-- 通知按钮 -->
    <Button
        android:id="@+id/btnNotifications"
        android:layout_width="0dp"
        android:layout_weight="1"
        android:layout_height="wrap_content"
        android:text="通知"
        android:drawableTop="@android:drawable/ic_dialog_info" />
</LinearLayout>
```

```kotlin
// MainActivity.kt 中添加
findViewById<Button>(R.id.btnNotifications)?.setOnClickListener {
    openNotificationList()
}

private fun openNotificationList() {
    val fragment = NotificationFragment()
    val args = Bundle()
    args.putInt(NotificationFragment.ARG_USER_ID, getCurrentUserId())
    fragment.arguments = args

    supportFragmentManager.beginTransaction()
        .replace(R.id.fragmentContainer, fragment)
        .addToBackStack("notifications")
        .commit()
}
```

## 配置选项

### 服务器地址配置

在`build.gradle.kts`中添加：

```kotlin
android {
    defaultConfig {
        // 添加服务器地址配置
        buildConfigField("String", "SERVER_URL", "\"${project.findProperty("serverUrl")}\"")
    }

    buildTypes {
        debug {
            buildConfigField("String", "SERVER_URL", "\"10.0.2.2:8080\"") // Android模拟器
        }
        release {
            buildConfigField("String", "SERVER_URL", "\"your-server.com:8080\"") // 生产环境
        }
    }
}
```

然后在`NotificationWebSocket.kt`中使用：

```kotlin
companion object {
    const val TAG = "NotificationWebSocket"
    val SERVER_URL = BuildConfig.SERVER_URL
}
```

### 加密密钥配置

在你的应用中配置加密密钥（与后端一致）：

```kotlin
// 在Application类中
class MyApplication : Application() {
    companion object {
        const val ENCRYPTION_KEY = "your-32-byte-hex-key"
    }

    override fun onCreate() {
        super.onCreate()
        // 初始化加密管理器
        AesGcmManager.initialize(this, ENCRYPTION_KEY)
    }
}
```

## 完整示例

### Application类

```kotlin
import android.app.Application

class MyApplication : Application() {
    companion object {
        private const val ENCRYPTION_KEY = "0123456789abcdef0123456789abcdef"
        lateinit var instance: MyApplication
            private set
    }

    override fun onCreate() {
        super.onCreate()
        instance = this

        // 初始化加密管理器
        try {
            AesGcmManager.initialize(this, ENCRYPTION_KEY)
        } catch (e: Exception) {
            android.util.Log.e("MyApplication", "Failed to initialize crypto", e)
        }
    }
}
```

### MainActivity（完整版）

```kotlin
package com.todoapp

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.widget.Button
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.todoapp.data.notify.NotificationManager
import com.todoapp.data.notify.NotificationWebSocket
import com.todoapp.ui.notifications.NotificationFragment
import kotlinx.coroutines.*
import org.json.JSONObject

class MainActivity : AppCompatActivity() {
    private lateinit var notificationManager: NotificationManager
    private lateinit var notificationWebSocket: NotificationWebSocket
    private val scope = CoroutineScope(Dispatchers.Main + SupervisorJob())

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        initializeNotificationSystem()
        setupNotificationsButton()
    }

    private fun initializeNotificationSystem() {
        val userId = getCurrentUserId()
        val token = getCurrentToken()

        if (userId > 0 && token.isNotEmpty()) {
            // 初始化通知管理器
            notificationManager = NotificationManager(this, userId)

            // 连接WebSocket
            notificationWebSocket = NotificationWebSocket(this, token, true)
            lifecycleScope.launch(Dispatchers.IO) {
                notificationWebSocket.connect()
            }

            // 注册消息处理器
            notificationWebSocket.onMessage("notification") { message ->
                handleNotificationMessage(message)
            }
        }
    }

    private fun setupNotificationsButton() {
        findViewById<Button>(R.id.btnNotifications)?.setOnClickListener {
            openNotificationList()
        }
    }

    private fun openNotificationList() {
        val fragment = NotificationFragment()
        val args = Bundle()
        args.putInt(NotificationFragment.ARG_USER_ID, getCurrentUserId())
        fragment.arguments = args

        supportFragmentManager.beginTransaction()
            .replace(R.id.fragmentContainer, fragment)
            .addToBackStack("notifications")
            .commit()
    }

    private fun getCurrentUserId(): Int {
        return getSharedPreferences("user_prefs", MODE_PRIVATE)
            .getInt("user_id", 0)
    }

    private fun getCurrentToken(): String {
        return getSharedPreferences("auth_prefs", MODE_PRIVATE)
            .getString("access_token", "") ?: ""
    }

    private fun handleNotificationMessage(message: JSONObject) {
        scope.launch {
            try {
                val type = message.optString("type", "system")
                val title = message.optString("title", "新通知")
                val content = message.optString("content", "")

                // 创建本地通知
                notificationManager.createNotification(type, title, content)

                // 显示Toast
                runOnUiThread {
                    Toast.makeText(this@MainActivity, title, Toast.LENGTH_SHORT).show()
                }
            } catch (e: Exception) {
                android.util.Log.e("MainActivity", "Failed to handle notification", e)
            }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        notificationWebSocket.disconnect()
        scope.cancel()
        notificationManager.cleanup()
    }
}
```

## 数据库迁移

当你添加了`Notification`实体到数据库后，需要提供迁移策略：

```kotlin
// AppDatabase.kt 中添加
@Database(
    entities = [Task::class, DeltaChange::class, SyncMeta::class, Conflict::class, Notification::class],
    version = 2, // 版本号增加
    exportSchema = false
)
abstract class AppDatabase : RoomDatabase() {
    // ... 现有代码

    companion object {
        @Volatile
        private var INSTANCE: AppDatabase? = null

        fun getInstance(context: Context): AppDatabase {
            return INSTANCE ?: synchronized(this) {
                val instance = Room.databaseBuilder(
                    context.applicationContext,
                    AppDatabase::class.java,
                    "todoapp.db"
                )
                .fallbackToDestructiveMigration() // 开发时使用，生产环境应使用proper migration
                .build()
                INSTANCE = instance
            }
        }
    }
}
```

## 测试

### 1. 测试通知创建

```kotlin
// 在MainActivity中添加测试函数
private fun testNotification() {
    lifecycleScope.launch {
        try {
            notificationManager.createNotification(
                type = "test",
                title = "测试通知",
                content = "这是一条测试通知",
                priority = "normal"
            )
            Toast.makeText(this@MainActivity, "通知创建成功", Toast.LENGTH_SHORT).show()
        } catch (e: Exception) {
            Toast.makeText(this@MainActivity, "通知创建失败", Toast.LENGTH_SHORT).show()
        }
    }
}
```

### 2. 测试WebSocket连接

```bash
# 启动应用后，查看Logcat
adb logcat | grep NotificationWebSocket

# 你应该看到：
# NotificationWebSocket: WebSocket connected
# NotificationWebSocket: WebSocket handshake completed, encryption: true
```

### 3. 测试系统集成

```bash
# 使用curl创建通知
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test",
    "title": "Android测试",
    "content": "发送到Android的通知",
    "priority": "high"
  }'

# 你应该能在Android应用中看到：
# 1. 系统通知
# 2. Toast提示
# 3. 通知列表中有新通知
```

## 故障排除

### 问题1：数据库迁移错误
**解决方案**：
```kotlin
// 使用fallbackToDestructiveMigration重新开始数据库
database = Room.databaseBuilder(...)
    .fallbackToDestructiveMigration()
    .build()
```

### 问题2：WebSocket连接失败
**解决方案**：
- 确认服务器地址正确（模拟器用10.0.2.2，真机用实际IP）
- 确认JWT token有效
- 确认网络权限已配置（AndroidManifest.xml）
- 检查防火墙设置

### 问题3：通知不显示
**解决方案**：
```xml
<!-- AndroidManifest.xml 中添加权限 -->
<uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
```

### 问题4：加密错误
**解决方案**：
- 确认加密密钥配置正确
- 确认密钥长度为32字节
- 确认前后端使用相同的密钥

## 性能优化

### 1. 使用协程优化

```kotlin
// 使用协程池
private val notificationScope = CoroutineScope(
    Dispatchers.IO + SupervisorJob() + Job()
)
```

### 2. 通知批量处理

```kotlin
// NotificationManager中添加
suspend fun createNotificationsBatch(notifications: List<NotificationData>) {
    withContext(Dispatchers.IO) {
        notifications.forEach { data ->
            createNotification(data.type, data.title, data.content, data.priority)
        }
    }
}
```

## 下一步

1. 配置Android SDK
2. 同步Gradle
3. 集成到MainActivity
4. 测试通知功能
5. 自定义样式和配置
6. 添加到生产环境

如有问题，请查看详细的代码复核报告：`CODE_REVIEW_REPORT.md`
