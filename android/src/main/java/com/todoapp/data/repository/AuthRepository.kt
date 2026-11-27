package com.todoapp.data.repository

import android.content.Context
import com.todoapp.data.remote.*
import com.todoapp.utils.JwtUtils
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthRepository @Inject constructor(
    // Hilt 只提供 @ApplicationContext 限定的 Context 绑定，裸 Context 无法注入
    @ApplicationContext private val context: Context
) {
    
    suspend fun login(email: String, password: String): Result<LoginResponse> {
        return withContext(Dispatchers.IO) {
            try {
                val request = LoginRequest(email, password)
                // 动态解析：切换服务器后立刻指向新地址，避免单例缓存导致请求仍打到旧服务端
                val response = RetrofitClient.getApiService(context).login(request)
                
                val loginBody = response.body()
                if (response.isSuccessful && loginBody != null) {
                    // Save tokens using RetrofitClient
                    RetrofitClient.saveAccessToken(context, loginBody.accessToken)
                    loginBody.refreshToken?.let { token ->
                        RetrofitClient.saveRefreshToken(context, token)
                    }

                    Result.success(loginBody)
                } else {
                    Result.failure(Exception("Login failed: ${response.code()}"))
                }
            } catch (e: Exception) {
                Result.failure(e)
            }
        }
    }
    
    suspend fun logout(): Result<Unit> {
        return withContext(Dispatchers.IO) {
            try {
                RetrofitClient.getApiService(context).logout()
            } catch (e: Exception) {
                // 离线或会话已失效时也允许本地登出
                android.util.Log.w("AuthRepository", "Server logout failed, clearing local session anyway", e)
            } finally {
                // 无论服务端登出是否成功，本地会话与每设备密钥都必须清除（换账号后需重新配对）
                RetrofitClient.clearToken(context)
                com.todoapp.data.crypto.KeyStorage.clearKey(context)
                com.todoapp.data.crypto.CryptoInterceptor.getInstance(context.applicationContext).refreshKey()
            }
            Result.success(Unit)
        }
    }
    
    suspend fun refreshToken(): Result<RefreshResponse> {
        return withContext(Dispatchers.IO) {
            try {
                val response = RetrofitClient.getApiService(context).refreshToken()
                
                val refreshBody = response.body()
                if (response.isSuccessful && refreshBody != null) {
                    RetrofitClient.saveAccessToken(context, refreshBody.accessToken)
                    Result.success(refreshBody)
                } else {
                    Result.failure(Exception("Token refresh failed: ${response.code()}"))
                }
            } catch (e: Exception) {
                Result.failure(e)
            }
        }
    }
    
    fun isLoggedIn(): Boolean {
        val token = RetrofitClient.getAccessToken(context)
        if (token.isEmpty()) return false
        // 与 TodoApp.isUserLoggedIn 保持一致：过期令牌交给 401 自动续期链路，仅非法格式/类型视为未登录
        return when (JwtUtils.validateAccessToken(token)) {
            is JwtUtils.TokenValidation.Valid,
            is JwtUtils.TokenValidation.Expired -> true
            else -> false
        }
    }
}