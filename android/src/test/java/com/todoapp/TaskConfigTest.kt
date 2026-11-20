package com.todoapp

import com.todoapp.config.EncryptionConfig
import com.todoapp.config.SyncConfig
import com.todoapp.config.TaskConfig
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class TaskConfigTest {

    @Test
    fun statuses_containFourStatesIncludingArchived() {
        assertEquals(listOf("todo", "in_progress", "done", "archived"), TaskConfig.STATUSES)
    }

    @Test
    fun priorities_containLowMediumHigh() {
        assertEquals(listOf("low", "medium", "high"), TaskConfig.PRIORITIES)
    }

    @Test
    fun defaults_areTodoAndMedium() {
        assertEquals("todo", TaskConfig.DEFAULT_STATUS)
        assertEquals("medium", TaskConfig.DEFAULT_PRIORITY)
    }

    @Test
    fun titleAndDescriptionLimits_matchBackend() {
        assertEquals(200, TaskConfig.MAX_TITLE_LENGTH)
        assertEquals(2000, TaskConfig.MAX_DESCRIPTION_LENGTH)
    }

    @Test
    fun encryptionConfig_aesGcm256() {
        assertEquals(256, EncryptionConfig.KEY_SIZE)
        assertEquals(12, EncryptionConfig.GCM_IV_LENGTH)
        assertEquals(128, EncryptionConfig.GCM_TAG_LENGTH)
        assertEquals("AES/GCM/NoPadding", EncryptionConfig.ALGORITHM)
    }

    @Test
    fun syncInterval_isPositive() {
        assertTrue(SyncConfig.SYNC_INTERVAL_MINUTES > 0)
        assertTrue(SyncConfig.MAX_RETRY_ATTEMPTS > 0)
    }
}
