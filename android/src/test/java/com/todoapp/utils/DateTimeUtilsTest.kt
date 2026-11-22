package com.todoapp.utils

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Date
import java.util.TimeZone

class DateTimeUtilsTest {

    private fun isoUtc(millis: Long): String {
        val format = java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'")
        format.timeZone = TimeZone.getTimeZone("UTC")
        return format.format(Date(millis))
    }

    @Test
    fun `getCurrentTimestamp 符合 ISO8601 UTC 格式`() {
        val timestamp = DateTimeUtils.getCurrentTimestamp()
        assertTrue(timestamp.matches(Regex("\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}Z")))
    }

    @Test
    fun `parseFromISO8601 正确解析合法时间戳`() {
        val date = DateTimeUtils.parseFromISO8601("2026-01-15T10:30:00Z")
        assertTrue(date != null)
        val parsed = date!!
        val calendar = java.util.Calendar.getInstance(TimeZone.getTimeZone("UTC"))
        calendar.time = parsed
        assertEquals(2026, calendar.get(java.util.Calendar.YEAR))
        assertEquals(0, calendar.get(java.util.Calendar.MONTH))
        assertEquals(15, calendar.get(java.util.Calendar.DAY_OF_MONTH))
        assertEquals(10, calendar.get(java.util.Calendar.HOUR_OF_DAY))
        assertEquals(30, calendar.get(java.util.Calendar.MINUTE))
    }

    @Test
    fun `parseFromISO8601 非法输入返回 null`() {
        assertNull(DateTimeUtils.parseFromISO8601("not-a-date"))
        assertNull(DateTimeUtils.parseFromISO8601(""))
        assertNull(DateTimeUtils.parseFromISO8601("2026/01/15"))
    }

    @Test
    fun `formatForDisplay 输出 MM-dd HH:mm`() {
        val display = DateTimeUtils.formatForDisplay("2026-01-15T10:30:00Z")
        assertTrue(display.matches(Regex("\\d{2}-\\d{2} \\d{2}:\\d{2}")))
    }

    @Test
    fun `formatForDisplay 非法输入返回空串`() {
        assertEquals("", DateTimeUtils.formatForDisplay("bad"))
        assertEquals("", DateTimeUtils.formatForDisplay(""))
    }

    @Test
    fun `formatForFullDisplay 输出完整时间格式`() {
        val display = DateTimeUtils.formatForFullDisplay("2026-01-15T10:30:00Z")
        assertTrue(display.matches(Regex("\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}")))
    }

    @Test
    fun `formatForFullDisplay 非法输入返回原文`() {
        assertEquals("bad-input", DateTimeUtils.formatForFullDisplay("bad-input"))
    }

    @Test
    fun `formatDateTime 拼接年月日时分`() {
        assertEquals("2026-01-15T08:05:00Z", DateTimeUtils.formatDateTime(2026, 0, 15, 8, 5))
    }

    @Test
    fun `formatDateTime 月份加一（0基月份入参）`() {
        assertEquals("2026-12-01T00:00:00Z", DateTimeUtils.formatDateTime(2026, 11, 1))
    }

    @Test
    fun `isExpired 过去时间为 true`() {
        val past = isoUtc(System.currentTimeMillis() - 60_000)
        assertTrue(DateTimeUtils.isExpired(past))
    }

    @Test
    fun `isExpired 未来时间为 false`() {
        val future = isoUtc(System.currentTimeMillis() + 60_000)
        assertFalse(DateTimeUtils.isExpired(future))
    }

    @Test
    fun `isExpired 非法时间视为过期`() {
        assertTrue(DateTimeUtils.isExpired("bad"))
        assertTrue(DateTimeUtils.isExpired(""))
    }

    @Test
    fun `getRelativeTime 一分钟内显示刚刚`() {
        val justNow = isoUtc(System.currentTimeMillis() - 30_000)
        assertEquals("刚刚", DateTimeUtils.getRelativeTime(justNow))
    }

    @Test
    fun `getRelativeTime 一小时内显示分钟前`() {
        val minutesAgo = isoUtc(System.currentTimeMillis() - 5 * 60_000)
        assertTrue(DateTimeUtils.getRelativeTime(minutesAgo).endsWith("分钟前"))
    }

    @Test
    fun `getRelativeTime 一天内显示小时前`() {
        val hoursAgo = isoUtc(System.currentTimeMillis() - 3 * 3_600_000)
        assertTrue(DateTimeUtils.getRelativeTime(hoursAgo).endsWith("小时前"))
    }

    @Test
    fun `getRelativeTime 一周内显示天前`() {
        val daysAgo = isoUtc(System.currentTimeMillis() - 2 * 86_400_000)
        assertTrue(DateTimeUtils.getRelativeTime(daysAgo).endsWith("天前"))
    }

    @Test
    fun `getRelativeTime 超过一周回退为日期显示`() {
        val longAgo = isoUtc(System.currentTimeMillis() - 10 * 86_400_000)
        val relative = DateTimeUtils.getRelativeTime(longAgo)
        assertTrue(relative.matches(Regex("\\d{2}-\\d{2} \\d{2}:\\d{2}")))
    }

    @Test
    fun `getRelativeTime 非法输入返回空串`() {
        assertEquals("", DateTimeUtils.getRelativeTime("bad"))
    }
}
