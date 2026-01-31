package com.todoapp.ui.auth

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.TodoApp
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class LoginFormState(
    val emailError: String? = null,
    val passwordError: String? = null,
    val isFormValid: Boolean = false
)

sealed class LoginState {
    object Idle : LoginState()
    object Loading : LoginState()
    data class Success(val token: String) : LoginState()
    data class Error(val message: String) : LoginState()
}

class LoginViewModel(application: Application) : AndroidViewModel(application) {
    
    private val context = getApplication<TodoApp>().applicationContext
    private val apiService: ApiService = RetrofitClient.getApiService(context)
    
    private val _loginState = MutableStateFlow<LoginState>(LoginState.Idle)
    val loginState: StateFlow<LoginState> = _loginState.asStateFlow()
    
    private val _formState = MutableStateFlow(LoginFormState())
    val formState: StateFlow<LoginFormState> = _formState.asStateFlow()

    fun login(email: String, password: String) {
        viewModelScope.launch {
            _loginState.value = LoginState.Loading
            
            try {
                val response = apiService.login(
                    com.todoapp.data.remote.ApiModels.LoginRequest(
                        email = email,
                        password = password
                    )
                )
                
                if (response.isSuccessful) {
                    val loginResponse = response.body()
                    if (loginResponse != null) {
                        // Store access token
                        RetrofitClient.saveAccessToken(context, loginResponse.access_token)
                        
                        // Schedule periodic sync
                        scheduleBackgroundSync()
                        
                        _loginState.value = LoginState.Success(loginResponse.access_token)
                    } else {
                        _loginState.value = LoginState.Error("登录响应为空")
                    }
                } else {
                    val errorBody = response.errorBody()?.string()
                    val errorMessage = parseErrorMessage(errorBody)
                    _loginState.value = LoginState.Error(errorMessage)
                }
            } catch (e: Exception) {
                _loginState.value = LoginState.Error("网络错误: ${e.message}")
            }
        }
    }

    fun validateEmail(email: String) {
        val currentState = _formState.value
        val emailError = if (email.isBlank()) {
            "邮箱不能为空"
        } else if (!isValidEmail(email)) {
            "邮箱格式不正确"
        } else {
            null
        }
        
        _formState.value = currentState.copy(
            emailError = emailError,
            isFormValid = emailError == null && currentState.passwordError == null
        )
    }

    fun validatePassword(password: String) {
        val currentState = _formState.value
        val passwordError = if (password.isBlank()) {
            "密码不能为空"
        } else if (password.length < 6) {
            "密码至少需要6个字符"
        } else {
            null
        }
        
        _formState.value = currentState.copy(
            passwordError = passwordError,
            isFormValid = passwordError == null && currentState.emailError == null
        )
    }

    fun validateForm(email: String, password: String) {
        val emailError = if (email.isBlank()) {
            "邮箱不能为空"
        } else if (!isValidEmail(email)) {
            "邮箱格式不正确"
        } else {
            null
        }
        
        val passwordError = if (password.isBlank()) {
            "密码不能为空"
        } else if (password.length < 6) {
            "密码至少需要6个字符"
        } else {
            null
        }
        
        _formState.value = LoginFormState(
            emailError = emailError,
            passwordError = passwordError,
            isFormValid = emailError == null && passwordError == null
        )
    }

    private fun isValidEmail(email: String): Boolean {
        return android.util.Patterns.EMAIL_ADDRESS.matcher(email).matches()
    }

    private fun parseErrorMessage(errorBody: String?): String {
        return try {
            if (errorBody != null) {
                // Try to parse JSON error message
                val gson = com.google.gson.Gson()
                val errorMap = gson.fromJson(errorBody, Map::class.java)
                errorMap["message"] as? String ?: "登录失败"
            } else {
                "登录失败"
            }
        } catch (e: Exception) {
            "登录失败"
        }
    }

    private fun scheduleBackgroundSync() {
        try {
            val workRequest = androidx.work.OneTimeWorkRequestBuilder<com.todoapp.data.sync.DeltaSyncWorker>()
                .setConstraints(
                    androidx.work.Constraints.Builder()
                        .setRequiredNetworkType(androidx.work.NetworkType.CONNECTED)
                        .build()
                )
                .build()
            
            androidx.work.WorkManager.getInstance(context).enqueue(workRequest)
        } catch (e: Exception) {
            // Handle error
        }
    }
}