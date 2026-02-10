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
                
                if (response.isSuccessful && response.body() != null) {
                    val loginResponse = response.body()!!
                    
                    // Save tokens using RetrofitClient
                    RetrofitClient.saveAccessToken(context, loginResponse.accessToken)
                    loginResponse.refreshToken?.let { token ->
                        RetrofitClient.saveRefreshToken(context, token)
                    }
                    
                    Result.success(loginResponse)
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
                val response = apiService.logout()
                RetrofitClient.clearToken(context)
                Result.success(Unit)
            } catch (e: Exception) {
                Result.failure(e)
            }
        }
    }
    
    suspend fun refreshToken(): Result<RefreshResponse> {
        return withContext(Dispatchers.IO) {
            try {
                val response = apiService.refreshToken()
                
                if (response.isSuccessful && response.body() != null) {
                    val refreshResponse = response.body()!!
                    RetrofitClient.saveAccessToken(context, refreshResponse.accessToken)
                    Result.success(refreshResponse)
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