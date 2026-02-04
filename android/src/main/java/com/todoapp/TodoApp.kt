package com.todoapp

import android.app.Application
import androidx.work.Configuration
import androidx.work.WorkManager
import com.todoapp.config.AppConfig
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.RetrofitClient
import dagger.hilt.android.HiltAndroidApp

@HiltAndroidApp
class TodoApp : Application(), Configuration.Provider {

    override fun onCreate() {
        super.onCreate()

        AppConfig.initialize(this)

        WorkManager.initialize(this, workManagerConfiguration)
    }

    override fun getWorkManagerConfiguration(): Configuration {
        return Configuration.Builder()
            .setMinimumLoggingLevel(if (BuildConfig.DEBUG) android.util.Log.DEBUG else android.util.Log.ERROR)
            .build()
    }

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