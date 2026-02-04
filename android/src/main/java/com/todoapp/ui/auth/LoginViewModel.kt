package com.todoapp.ui.auth

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.R
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.LoginRequest
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.Result
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

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

@HiltViewModel
class LoginViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val apiService: ApiService
) : ViewModel() {

    private val _loginState = MutableStateFlow<LoginState>(LoginState.Idle)
    val loginState: StateFlow<LoginState> = _loginState.asStateFlow()

    private val _formState = MutableStateFlow(LoginFormState())
    val formState: StateFlow<LoginFormState> = _formState.asStateFlow()

    fun login(email: String, password: String) {
        viewModelScope.launch {
            _loginState.value = LoginState.Loading

            val result = Result.runCatching {
                val response = apiService.login(
                    LoginRequest(
                        email = email,
                        password = password
                    )
                )

                if (!response.isSuccessful) {
                    val errorBody = response.errorBody()?.string()
                    throw Exception(parseErrorMessage(errorBody))
                }

                val loginResponse = response.body() ?: throw Exception(
                    context.getString(R.string.login_response_empty)
                )

                RetrofitClient.saveAccessToken(context, loginResponse.accessToken)
                loginResponse.refreshToken?.let { refreshToken ->
                    RetrofitClient.saveRefreshToken(context, refreshToken)
                }

                loginResponse.accessToken
            }

            result.onSuccess { token ->
                _loginState.value = LoginState.Success(token)
            }.onError { e ->
                _loginState.value = LoginState.Error(e.message ?: context.getString(R.string.login_failed))
            }
        }
    }

    fun validateEmail(email: String) {
        val currentState = _formState.value
        val emailError = when {
            email.isBlank() -> context.getString(R.string.login_email_empty)
            !isValidEmail(email) -> context.getString(R.string.login_email_invalid)
            else -> null
        }

        _formState.value = currentState.copy(
            emailError = emailError,
            isFormValid = emailError == null && currentState.passwordError == null
        )
    }

    fun validatePassword(password: String) {
        val currentState = _formState.value
        val passwordError = when {
            password.isBlank() -> context.getString(R.string.login_password_empty)
            password.length < 6 -> context.getString(R.string.login_password_short)
            else -> null
        }

        _formState.value = currentState.copy(
            passwordError = passwordError,
            isFormValid = passwordError == null && currentState.emailError == null
        )
    }

    fun validateForm(email: String, password: String) {
        val emailError = when {
            email.isBlank() -> context.getString(R.string.login_email_empty)
            !isValidEmail(email) -> context.getString(R.string.login_email_invalid)
            else -> null
        }

        val passwordError = when {
            password.isBlank() -> context.getString(R.string.login_password_empty)
            password.length < 6 -> context.getString(R.string.login_password_short)
            else -> null
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
                val gson = com.google.gson.Gson()
                val errorMap = gson.fromJson(errorBody, Map::class.java)
                errorMap["message"] as? String ?: context.getString(R.string.login_failed)
            } else {
                context.getString(R.string.login_failed)
            }
        } catch (e: Exception) {
            context.getString(R.string.login_failed)
        }
    }
}
