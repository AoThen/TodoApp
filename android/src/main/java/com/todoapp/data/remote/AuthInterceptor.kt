package com.todoapp.data.remote

import okhttp3.Interceptor
import okhttp3.Response

/**
 * 鉴权拦截器：注入访问令牌；响应 401 时在锁内尝试刷新令牌并重试，
 * 刷新结果按语义处理（成功重试 / 会话失效清会话 / 瞬态失败保留原响应）。
 * 依赖以函数注入，便于单元测试覆盖并发竞态分支。
 */
internal class AuthInterceptor(
    private val getToken: () -> String?,
    private val clearToken: () -> Unit,
    private val tryRefresh: () -> RefreshOutcome,
    private val onSessionExpired: () -> Unit,
    private val lock: Any
) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val originalRequest = chain.request()

        val token = getToken()

        val requestWithAuth = originalRequest.newBuilder().apply {
            addHeader("Accept", "application/json")
            header("Content-Type", "application/json")
            token?.let {
                addHeader("Authorization", "Bearer $it")
            }
        }.build()

        val response = chain.proceed(requestWithAuth)

        // 登录接口的 401 表示凭据错误，直接透传给调用方展示
        val isLoginRequest = originalRequest.url.encodedPath.endsWith("/auth/login")
        if (response.code != 401 || isLoginRequest) {
            return response
        }

        synchronized(lock) {
            val currentToken = getToken()
            // 拿锁后发现 token 已被其他线程刷新时无需再次刷新，直接用最新 token 重试；
            // 否则低概率下后来者会拿着新 token 仍收到 401，造成偶发同步失败/被登出假象
            val outcome = if (currentToken == token) {
                tryRefresh()
            } else {
                RefreshOutcome.Success
            }
            when (outcome) {
                RefreshOutcome.Success -> {
                    response.close()
                    val newToken = getToken()
                    val retryRequest = originalRequest.newBuilder().apply {
                        addHeader("Accept", "application/json")
                        header("Content-Type", "application/json")
                        newToken?.let {
                            addHeader("Authorization", "Bearer $it")
                        }
                    }.build()

                    return chain.proceed(retryRequest)
                }
                RefreshOutcome.InvalidSession -> {
                    // 服务端明确拒绝刷新令牌：清除本地会话并广播失效事件，
                    // 由 UI 层引导重新登录；离线等瞬态失败不动会话，避免误踢
                    android.util.Log.e("RetrofitClient", "Refresh token rejected, session expired")
                    clearToken()
                    onSessionExpired()
                }
                RefreshOutcome.TransientError -> {
                    android.util.Log.e("RetrofitClient", "Token refresh failed transiently, keep session")
                }
            }
        }

        // 刷新失败：保留原始响应（不能 close 后再返回，否则调用方读取错误体时报 closed）
        response
    }
}
