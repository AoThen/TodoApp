package com.todoapp.data.remote

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

object RetrofitClient {
    private var BASE_URL = "http://10.0.2.2:8080/api/v1/"
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
            .addInterceptor(createAuthInterceptor(context))
            .addInterceptor(createLoggingInterceptor())
            .connectTimeout(CONNECT_TIMEOUT, TimeUnit.SECONDS)
            .readTimeout(READ_TIMEOUT, TimeUnit.SECONDS)
            .writeTimeout(WRITE_TIMEOUT, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build()
    }

    private fun createAuthInterceptor(context: Context): Interceptor {
        return Interceptor { chain ->
            val originalRequest = chain.request()

            val prefs = getEncryptedPrefs(context)
            val token = prefs.getString(ACCESS_TOKEN_KEY, null)

            val requestWithAuth = originalRequest.newBuilder().apply {
                addHeader("Accept", "application/json")
                addHeader("Content-Type", "application/json")
                token?.let {
                    addHeader("Authorization", "Bearer $it")
                }
            }.build()

            val response = chain.proceed(requestWithAuth)

            if (response.code == 401) {
                response.close()

                synchronized(refreshLock) {
                    val currentToken = prefs.getString(ACCESS_TOKEN_KEY, null)
                    if (currentToken == token && tryRefreshToken(context)) {
                        val newToken = prefs.getString(ACCESS_TOKEN_KEY, null)
                        val retryRequest = originalRequest.newBuilder().apply {
                            addHeader("Accept", "application/json")
                            addHeader("Content-Type", "application/json")
                            newToken?.let {
                                addHeader("Authorization", "Bearer $it")
                            }
                        }.build()

                        return@Interceptor chain.proceed(retryRequest)
                    }
                }

                android.util.Log.e("RetrofitClient", "Token refresh failed, user needs to re-login")
            }

            response
        }
    }

    private fun tryRefreshToken(context: Context): Boolean {
        return try {
            val prefs = getEncryptedPrefs(context)
            val refreshToken = prefs.getString(REFRESH_TOKEN_KEY, null)

            if (refreshToken.isNullOrEmpty()) {
                return false
            }

            val tempClient = OkHttpClient.Builder()
                .addInterceptor(createLoggingInterceptor())
                .connectTimeout(10, TimeUnit.SECONDS)
                .readTimeout(10, TimeUnit.SECONDS)
                .writeTimeout(10, TimeUnit.SECONDS)
                .build()

            val tempRetrofit = Retrofit.Builder()
                .baseUrl(BASE_URL)
                .client(tempClient)
                .addConverterFactory(GsonConverterFactory.create())
                .build()

            val tempApiService = tempRetrofit.create(ApiService::class.java)

            val response = runBlocking {
                tempApiService.refreshToken()
            }

            if (response.isSuccessful && response.body() != null) {
                val refreshResponse = response.body()!!
                saveAccessToken(context, refreshResponse.accessToken)
                android.util.Log.d("RetrofitClient", "Token refreshed successfully")
                true
            } else {
                android.util.Log.e("RetrofitClient", "Token refresh failed: ${response.code()}")
                false
            }
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Token refresh error", e)
            false
        }
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
        return try {
            val masterKey = MasterKey.Builder(context)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()

            EncryptedSharedPreferences.create(
                context,
                PREFS_NAME,
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        } catch (e: Exception) {
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
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
        } catch (e: Exception) {
            android.util.Log.e("RetrofitClient", "Failed to clear tokens", e)
        }
    }

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
    }

    fun getBaseUrl(): String {
        return BASE_URL
    }
}
