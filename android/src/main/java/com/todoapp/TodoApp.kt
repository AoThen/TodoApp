package com.todoapp

import android.app.Application
import androidx.work.Configuration
import com.todoapp.config.AppConfig
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.ThemeUtils
import dagger.hilt.android.HiltAndroidApp

@HiltAndroidApp
class TodoApp : Application(), Configuration.Provider {

    override fun onCreate() {
        super.onCreate()

        KeyStorage.init()
        AppConfig.initialize(this)
        // 进程重启后恢复上次配对的服务端地址，避免登录/同步仍指向构建期默认地址
        RetrofitClient.setBaseUrl(AppConfig.getBaseUrl())
        ThemeUtils.applySavedMode(this)
    }

    // WorkManager 2.9+ 的 Configuration.Provider 使用属性而非 getter 方法
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder()
            .setMinimumLoggingLevel(if (BuildConfig.DEBUG) android.util.Log.DEBUG else android.util.Log.ERROR)
            .build()

    fun isUserPaired(): Boolean {
        return try {
            KeyStorage.hasValidPairing(this)
        } catch (e: Exception) {
            false
        }
    }

    fun isUserLoggedIn(): Boolean {
        return RetrofitClient.hasToken(this) &&
        RetrofitClient.getAccessToken(this).isNotEmpty()
    }
}