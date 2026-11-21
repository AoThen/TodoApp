package com.todoapp.utils

import android.content.Context
import androidx.appcompat.app.AppCompatDelegate

/** 深色/浅色外观偏好：本地持久化 + 立即应用到全局 */
object ThemeUtils {
    private const val PREFS_NAME = "app_settings"
    private const val KEY_DARK_MODE = "dark_mode"
    private const val KEY_DARK_MODE_SET = "dark_mode_set"

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    /** 用户是否显式设置过外观；未设置时应用跟随系统 */
    fun isDarkModeSet(context: Context): Boolean =
        prefs(context).getBoolean(KEY_DARK_MODE_SET, false)

    fun isDarkMode(context: Context): Boolean =
        prefs(context).getBoolean(KEY_DARK_MODE, false)

    /** 保存并立即应用外观模式 */
    fun setDarkMode(context: Context, darkMode: Boolean) {
        prefs(context).edit()
            .putBoolean(KEY_DARK_MODE, darkMode)
            .putBoolean(KEY_DARK_MODE_SET, true)
            .apply()
        applySavedMode(context)
    }

    /** 启动时应用外观偏好：未设置时跟随系统，设置过则按保存值 */
    fun applySavedMode(context: Context) {
        val mode = when {
            !isDarkModeSet(context) -> AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM
            isDarkMode(context) -> AppCompatDelegate.MODE_NIGHT_YES
            else -> AppCompatDelegate.MODE_NIGHT_NO
        }
        AppCompatDelegate.setDefaultNightMode(mode)
    }
}
