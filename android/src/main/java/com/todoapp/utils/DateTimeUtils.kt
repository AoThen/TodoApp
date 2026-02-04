package com.todoapp.utils

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

object DateTimeUtils {
    private const val ISO_8601_FORMAT = "yyyy-MM-dd'T'HH:mm:ss'Z'"
    private const val DISPLAY_FORMAT = "MM-dd HH:mm"
    private const val FULL_DISPLAY_FORMAT = "yyyy-MM-dd HH:mm:ss"

    fun getCurrentTimestamp(): String {
        return SimpleDateFormat(ISO_8601_FORMAT, Locale.US).apply {
            timeZone = TimeZone.getTimeZone("UTC")
        }.format(Date())
    }

    fun formatForDisplay(timestamp: String): String {
        return try {
            val date = parseFromISO8601(timestamp) ?: return ""
            SimpleDateFormat(DISPLAY_FORMAT, Locale.getDefault()).format(date)
        } catch (e: Exception) {
            ""
        }
    }

    fun formatForFullDisplay(timestamp: String): String {
        return try {
            val date = parseFromISO8601(timestamp) ?: return timestamp
            SimpleDateFormat(FULL_DISPLAY_FORMAT, Locale.getDefault()).format(date)
        } catch (e: Exception) {
            timestamp
        }
    }

    fun parseFromISO8601(timestamp: String): Date? {
        return try {
            SimpleDateFormat(ISO_8601_FORMAT, Locale.US).apply {
                timeZone = TimeZone.getTimeZone("UTC")
            }.parse(timestamp)
        } catch (e: Exception) {
            null
        }
    }

    fun formatDateTime(year: Int, month: Int, day: Int, hour: Int = 0, minute: Int = 0): String {
        return String.format(
            Locale.US,
            "%04d-%02d-%02dT%02d:%02d:00Z",
            year, month + 1, day, hour, minute
        )
    }

    fun isExpired(timestamp: String): Boolean {
        return try {
            val date = parseFromISO8601(timestamp) ?: return true
            date.before(Date())
        } catch (e: Exception) {
            true
        }
    }

    fun getRelativeTime(timestamp: String): String {
        return try {
            val date = parseFromISO8601(timestamp) ?: return ""
            val now = System.currentTimeMillis()
            val diff = now - date.time

            val seconds = diff / 1000
            val minutes = seconds / 60
            val hours = minutes / 60
            val days = hours / 24

            when {
                seconds < 60 -> "刚刚"
                minutes < 60 -> "${minutes}分钟前"
                hours < 24 -> "${hours}小时前"
                days < 7 -> "${days}天前"
                else -> formatForDisplay(timestamp)
            }
        } catch (e: Exception) {
            ""
        }
    }
}