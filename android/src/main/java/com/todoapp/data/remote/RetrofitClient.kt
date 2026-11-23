package com.todoapp.data.remote

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

object RetrofitClient {

    /** 刷新令牌结果：Success 成功续期；InvalidSession 服务端明确拒绝（需重新登录）；TransientError 网络等瞬态失败 */
    enum class RefreshOutcome { Success, InvalidSession, TransientError }

    private var BASE_URL = com.todoapp.config.AppConfig.DEFAULT_BASE_URL
    private const val PREFS_NAME = "todoapp_prefs"
    private const val ACCESS_TOKEN_KEY = "access_token"
    private const val REFRESH_TOKEN_KEY = "refresh_token"

    private const val CONNECT_TIMEOUT = 30L
    private const val READ_TIMEOUT = 30L
    private const val WRITE_TIMEOUT = 30L

    @Volatile
    private var retrofit: Retrofit? = null

    @Volatile
    private var apiService: ApiService? = null

    @Volatile
    private var refreshRetrofit: Retrofit? = null

    @Volatile
    private var cachedPrefs: SharedPreferences? = null

    @Volatile
    private var cookieJar: PersistentCookieJar? = null

    private val refreshLock = Any()

    fun getApiService(context: Context): ApiService {
        return apiService ?: synchronized(this) {
            apiService ?: getRetrofit(context).create(ApiService::class.java).also {
                apiService = it
            }
        }
    }

    private fun getRetrofit(context: Context): Retrofit {
        return retrofit ?: synchronized(this) {
            retrofit ?: Retrofit.Builder()
                .baseUrl(BASE_URL)
                .client(createOkHttpClient(context))
                .addConverterFactory(GsonConverterFactory.create())
                .build()
                .also { retrofit = it }
        }
    }

    private fun createOkHttpClient(context: Context): OkHttpClient {
        return OkHttpClient.Builder()
            .cookieJar(getCookieJar(context.applicationContext))
            // 加密拦截器置于最外层：先加密请求体、标记加密头，再由鉴权/日志拦截器处理
            .addInterceptor(com.todoapp.data.crypto.CryptoInterceptor.getInstance(context.applicationContext))
            .addInterceptor(createAuthInterceptor(context))
            .addInterceptor(createLoggingInterceptor())
            .connectTimeout(CONNECT_TIMEOUT, TimeUnit.SECONDS)
            .readTimeout(READ_TIMEOUT, TimeUnit.SECONDS)
            .writeTimeout(WRITE_TIMEOUT, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build()
    }

    private fun getCookieJar(appContext: Context): PersistentCookieJar {
        return cookieJar ?: synchronized(this) {
            cookieJar ?: PersistentCookieJar(appContext).also { cookieJar = it }
        }
    }

    private fun createAuthInterceptor(context: Context): Interceptor {
        // 拦截器随 Retrofit 单例长期存活，仅捕获 applicationContext 避免持有 Activity
        val appContext = context.applicationContext
        return AuthInterceptor(
            getToken = { getEncryptedPrefs(appContext).getString(ACCESS_TOKEN_KEY, null) },
            clearToken = { clearToken(appContext) },
            tryRefresh = { tryRefreshToken(appContext) },
            onSessionExpired = { SessionEvents.notifyExpired() },
            lock = refreshLock
        )
    }

    /**
     * 主动刷新访问令牌（如启动时检测到过期）。
     * 与拦截器共用 refreshLock，避免并发触发服务端一次性 refresh token 轮换互踢。
     */
    fun refreshAccessToken(context: Context): RefreshOutcome {
        return synchronized(refreshLock) {
            tryRefreshToken(context)
        }
    }

    internal fun tryRefreshToken(context: Context): RefreshOutcome {
        return try {
            // refresh token 由服务端通过 HttpOnly Cookie 下发，随 CookieJar 自动携带；
            // 无 Cookie 时服务端返回 401，本方法返回 InvalidSession
            // 注意：elvis 表达式必须加括号后再调用 create，否则公共类型退化为 Any
            val refreshApiService = (refreshRetrofit ?: synchronized(this) {
                refreshRetrofit ?: Retrofit.Builder()
                    .baseUrl(BASE_URL)
                    .client(createRefreshOkHttpClient(context.applicationContext))
                    .addConverterFactory(GsonConverterFactory.create())
                    .build()
                    .also { refreshRetrofit = it }
            }).create(ApiService::class.java)

            val response = runBlocking {
                withTimeout(5000L) {
                    refreshApiService.refreshToken()
                }
            }

            val refreshedBody = response.body()
            if (response.isSuccessful && refreshedBody != null) {
                saveAccessToken(context, refreshedBody.accessToken)
                android.util.Log.d("RetrofitClient", "Token refreshed successfully")
                RefreshOutcome.Success
            } else {
                android.util.Log.e("RetrofitClient", "Token refresh failed: ${response.code()}")
                // 401/403 表示 refresh token 已失效或被拒绝；其余（5xx 等）视为瞬态失败
                if (response.code() == 401 || response.code() == 403) {
                    RefreshOutcome.InvalidSession
                } else {
                    RefreshOutcome.TransientError
                }
            }
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Token refresh error", e)
            RefreshOutcome.TransientError
        }
    }

    private fun createRefreshOkHttpClient(appContext: Context): OkHttpClient {
        return OkHttpClient.Builder()
            .cookieJar(getCookieJar(appContext))
            .addInterceptor(createLoggingInterceptor())
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(10, TimeUnit.SECONDS)
            .writeTimeout(10, TimeUnit.SECONDS)
            .build()
    }

    private fun createLoggingInterceptor(): HttpLoggingInterceptor {
        return HttpLoggingInterceptor().apply {
            level = if (com.todoapp.BuildConfig.DEBUG) {
                HttpLoggingInterceptor.Level.BODY
            } else {
                HttpLoggingInterceptor.Level.NONE
            }
        }
    }

    private fun getEncryptedPrefs(context: Context): SharedPreferences {
        cachedPrefs?.let { return it }
        return synchronized(this) {
            cachedPrefs?.let { return it }
            val appContext = context.applicationContext
            val prefs = try {
                val masterKeyAlias = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)

                EncryptedSharedPreferences.create(
                    PREFS_NAME,
                    masterKeyAlias,
                    appContext,
                    EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                    EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
                )
            } catch (e: Exception) {
                android.util.Log.e("RetrofitClient", "Failed to create EncryptedSharedPreferences, falling back to plain text", e)
                appContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            }
            cachedPrefs = prefs
            prefs
        }
    }

    fun saveAccessToken(context: Context, token: String) {
        try {
            val prefs = getEncryptedPrefs(context)
            prefs.edit().putString(ACCESS_TOKEN_KEY, token).apply()
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Failed to save access token", e)
        }
    }

    fun saveRefreshToken(context: Context, token: String) {
        try {
            val prefs = getEncryptedPrefs(context)
            prefs.edit().putString(REFRESH_TOKEN_KEY, token).apply()
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Failed to save refresh token", e)
        }
    }

    fun clearToken(context: Context) {
        try {
            val prefs = getEncryptedPrefs(context)
            prefs.edit()
                .remove(ACCESS_TOKEN_KEY)
                .remove(REFRESH_TOKEN_KEY)
                .apply()
            cookieJar?.clear()
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Failed to clear tokens", e)
        }
    }

    /** 仅判断本地是否存有 token 字符串（不含有效性校验）；登录态判定请用 JwtUtils.validateAccessToken */
    fun hasToken(context: Context): Boolean {
        return try {
            val prefs = getEncryptedPrefs(context)
            !prefs.getString(ACCESS_TOKEN_KEY, null).isNullOrEmpty()
        } catch (e: Exception) {
            false
        }
    }

    fun getAccessToken(context: Context): String {
        return try {
            val prefs = getEncryptedPrefs(context)
            prefs.getString(ACCESS_TOKEN_KEY, "") ?: ""
        } catch (e: Exception) {
            ""
        }
    }

    fun setBaseUrl(baseUrl: String) {
        BASE_URL = if (baseUrl.endsWith("/")) {
            if (!baseUrl.endsWith("/api/v1/")) {
                baseUrl + "api/v1/"
            } else {
                baseUrl
            }
        } else {
            baseUrl + "/api/v1/"
        }

        retrofit = null
        apiService = null
        refreshRetrofit = null
        // 切换服务端后旧会话凭据与 Cookie 均失效，一并清理
        cachedPrefs = null
        cookieJar?.clear()
    }

    fun getBaseUrl(): String {
        return BASE_URL
    }
}
