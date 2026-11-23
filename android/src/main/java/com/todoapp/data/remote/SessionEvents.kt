package com.todoapp.data.remote

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow

/**
 * 全局会话失效事件：网络层 refresh token 被服务端明确拒绝（401/403）时发出，
 * UI 层（MainActivity）订阅后清除会话并引导用户重新登录。
 *
 * 网络异常等瞬态失败不发出本事件，避免离线场景误踢用户。
 */
object SessionEvents {

    private val _expired = MutableSharedFlow<Unit>(
        extraBufferCapacity = 1,
        onBufferOverflow = kotlinx.coroutines.channels.BufferOverflow.DROP_OLDEST
    )
    val expired: SharedFlow<Unit> = _expired.asSharedFlow()

    fun notifyExpired() {
        _expired.tryEmit(Unit)
    }
}
