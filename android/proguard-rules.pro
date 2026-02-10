# 保留 Retrofit 模型和API
-keep class com.todoapp.data.remote.** { *; }
-keep class com.todoapp.data.repository.** { *; }

# 保留 Room 实体和DAO
-keep @androidx.room.Entity class *
-keep @androidx.room.Dao class *
-dontwarn com.todoapp.data.entities.**
-dontwarn com.todoapp.data.dao.**
-dontwarn com.todoapp.data.database.**

# 保留 Gson
-keepattributes Signature
-keepattributes *Annotation*
-dontwarn sun.misc.**

# 保留 OkHttp
-dontwarn okhttp3.**
-keep class okhttp3.** { *; }

# 保留 WorkManager
-keep class androidx.work.** { *; }

# 保留 Hilt 注入相关
-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.HiltAndroidApp { *; }
-keepclasseswithmembers class * {
    @dagger.hilt.android.AndroidEntryPoint <methods>;
}

# 保留 Fragment 和 ViewModel
-keep class * extends androidx.fragment.app.Fragment { *; }
-keep class * extends androidx.lifecycle.ViewModel { *; }

# 保留 ViewBinding
-keep public class * extends androidx.viewbinding.ViewBinding {
    public static *** inflate(android.view.LayoutInflater);
    public static *** inflate(android.view.LayoutInflater, android.view.ViewGroup, boolean);
    public static *** bind(android.view.View);
}

# 保留加密相关
-keep class androidx.security.crypto.** { *; }
-dontwarn androidx.security.crypto.**

# 移除日志（仅限发布版本）
-assumenosideeffects class android.util.Log {
    public static boolean isLoggable(java.lang.String, int);
    public static int v(...);
    public static int d(...);
    public static int i(...);
}
