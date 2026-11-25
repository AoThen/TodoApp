package com.todoapp.data.remote

import android.content.Context
import com.todoapp.data.crypto.CryptoInterceptor
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.local.AppDatabase

/**
 * 切换服务器时的会话重置：旧服务器的登录凭据、设备配对密钥与待同步修改全部作废，
 * 防止旧账号数据以新账号身份上传到新服务器造成跨账号/跨服务器串写。
 * 设置页修改地址与配对页自定义服务器两条路径共用，保证行为一致。
 */
object ServerSwitchHelper {

    /**
     * @param clearPendingDeltas 是否清空待同步队列（已登录时必须清；游客态传 false 保留，登录后仍可合并上传）
     */
    suspend fun resetSession(appContext: Context, clearPendingDeltas: Boolean) {
        if (clearPendingDeltas) {
            AppDatabase.getInstance(appContext).deltaQueueDao().deleteAllDeltas()
        }
        RetrofitClient.clearToken(appContext)
        KeyStorage.clearKey(appContext)
        CryptoInterceptor.getInstance(appContext).refreshKey()
    }
}
