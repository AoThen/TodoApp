package com.todoapp

import android.app.Application
import androidx.work.Configuration
import androidx.work.WorkManager
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.local.AppDatabase
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.sync.DeltaSyncWorker

class TodoApp : Application(), Configuration.Provider {

    // Database instance
    val database: AppDatabase by lazy {
        AppDatabase.getDatabase(this)
    }

    // Repository instances
    val retrofitClient by lazy {
        RetrofitClient
    }

    override fun onCreate() {
        super.onCreate()
        
        // Initialize WorkManager for background sync
        WorkManager.initialize(this, workManagerConfiguration)
    }

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder()
            .setMinimumLoggingLevel(if (BuildConfig.DEBUG) android.util.Log.DEBUG else android.util.Log.ERROR)
            .build()

    /**
     * Check if user is already paired (has encryption key)
     */
    fun isUserPaired(): Boolean {
        return try {
            KeyStorage.hasValidPairing(this)
        } catch (e: Exception) {
            false
        }
    }

    /**
     * Check if user is logged in (has access token)
     */
    fun isUserLoggedIn(): Boolean {
        return retrofitClient.hasToken(this) && 
        retrofitClient.getAccessToken(this).isNotEmpty()
    }
}