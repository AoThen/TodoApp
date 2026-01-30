package com.todoapp.data.remote

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

object RetrofitClient {
    // Base URL - should be configured via BuildConfig in production
    private const val BASE_URL = "http://10.0.2.2:8080/api/v1/"
    private const val PREFS_NAME = "todoapp_prefs"
    private const val ACCESS_TOKEN_KEY = "access_token"

    // Timeout configurations
    private const val CONNECT_TIMEOUT = 30L
    private const val READ_TIMEOUT = 30L
    private const val WRITE_TIMEOUT = 30L

    @Volatile
    private var retrofit: Retrofit? = null

    @Volatile
    private var apiService: ApiService? = null

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
            .retryOnConnectionFailure(true) // Auto-retry on connection failures
            .build()
    }

    private fun createAuthInterceptor(context: Context): Interceptor {
        return Interceptor { chain ->
            val request = try {
                val prefs = getEncryptedPrefs(context)
                val token = prefs.getString(ACCESS_TOKEN_KEY, null)

                chain.request().newBuilder().apply {
                    addHeader("Accept", "application/json")
                    addHeader("Content-Type", "application/json")
                    token?.let {
                        addHeader("Authorization", "Bearer $it")
                    }
                }.build()
            } catch (e: Exception) {
                // If encrypted prefs fail, try without auth
                chain.request().newBuilder()
                    .addHeader("Accept", "application/json")
                    .addHeader("Content-Type", "application/json")
                    .build()
            }

            val response = chain.proceed(request)

            // Handle 401 - attempt token refresh
            if (response.code == 401) {
                response.close()
                // In production, implement token refresh logic here
            }

            response
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
            // Fallback to regular shared preferences if encryption fails
            context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        }
    }

    fun saveAccessToken(context: Context, token: String) {
        try {
            val prefs = getEncryptedPrefs(context)
            prefs.edit().putString(ACCESS_TOKEN_KEY, token).apply()
        } catch (e: Exception) {
            // Log error in production
        }
    }

    fun clearToken(context: Context) {
        try {
            val prefs = getEncryptedPrefs(context)
            prefs.edit().remove(ACCESS_TOKEN_KEY).apply()
        } catch (e: Exception) {
            // Log error in production
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
}
