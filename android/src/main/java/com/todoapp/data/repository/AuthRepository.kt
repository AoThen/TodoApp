package com.todoapp.data.repository

import android.content.Context
import com.todoapp.data.remote.*
import com.todoapp.data.remote.RetrofitClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthRepository @Inject constructor(
    private val apiService: ApiService,
    private val context: Context
) {
    
    suspend fun login(email: String, password: String): Result<LoginResponse> {
        return withContext(Dispatchers.IO) {
            try {
                val request = LoginRequest(email, password)
                val response = apiService.login(request)
                
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
                apiService.logout()
            } catch (e: Exception) {
                // 离线或会话已失效时也允许本地登出
                android.util.Log.w("AuthRepository", "Server logout failed, clearing local session anyway", e)
            } finally {
                // 无论服务端登出是否成功，本地会话都必须清除
                RetrofitClient.clearToken(context)
            }
            Result.success(Unit)
        }
    }
    
    suspend fun refreshToken(): Result<RefreshResponse> {
        return withContext(Dispatchers.IO) {
            try {
                val response = apiService.refreshToken()
                
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
        return RetrofitClient.hasToken(context) && 
               RetrofitClient.getAccessToken(context).isNotEmpty()
    }
}