# 保留 Retrofit 模型
-keep class com.todoapp.data.remote.** { *; }

# 保留 Room 实体
-keep @androidx.room.Entity class *
-dontwarn com.todoapp.data.local.**

# 保留 Gson
-keepattributes Signature
-keepattributes *Annotation*
-dontwarn sun.misc.**

# 保留 OkHttp
-dontwarn okhttp3.**
-keep class okhttp3.** { *; }

# 保留 WorkManager
-keep class androidx.work.** { *; }

# 移除日志（仅限发布版本）
-assumenosideeffects class android.util.Log {
    public static boolean isLoggable(java.lang.String, int);
    public static int v(...);
    public static int d(...);
    public static int i(...);
}
