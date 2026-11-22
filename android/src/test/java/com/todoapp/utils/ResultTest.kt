package com.todoapp.utils

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class ResultTest {

    @Test
    fun `Success 状态判断正确`() {
        val result: Result<Int> = Result.Success(1)
        assertTrue(result.isSuccess)
        assertFalse(result.isError)
        assertFalse(result.isLoading)
    }

    @Test
    fun `Error 状态判断正确`() {
        val result: Result<Int> = Result.Error(IllegalStateException("boom"))
        assertFalse(result.isSuccess)
        assertTrue(result.isError)
        assertFalse(result.isLoading)
    }

    @Test
    fun `Loading 状态判断正确`() {
        val result: Result<Int> = Result.Loading
        assertFalse(result.isSuccess)
        assertFalse(result.isError)
        assertTrue(result.isLoading)
    }

    @Test
    fun `getOrNull 仅在 Success 返回数据`() {
        assertEquals(1, Result.Success(1).getOrNull())
        assertNull(Result.Error(IllegalStateException("x")).getOrNull())
        assertNull(Result.Loading.getOrNull())
    }

    @Test
    fun `exceptionOrNull 仅在 Error 返回异常`() {
        val ex = IllegalStateException("boom")
        assertSame(ex, Result.Error(ex).exceptionOrNull())
        assertNull(Result.Success(1).exceptionOrNull())
        assertNull(Result.Loading.exceptionOrNull())
    }

    @Test
    fun `map 仅转换 Success 数据`() {
        assertEquals(2, Result.Success(1).map { it + 1 }.getOrNull())
        val ex = IllegalStateException("x")
        val error: Result<Int> = Result.Error(ex)
        assertSame(ex, error.map { it + 1 }.exceptionOrNull())
        assertEquals(Result.Loading, Result.Loading.map { 0 })
    }

    @Test
    fun `onSuccess 仅在 Success 时回调并返回自身`() {
        var captured: Int? = null
        val result: Result<Int> = Result.Success(7)
        assertSame(result, result.onSuccess { captured = it })
        assertEquals(7, captured)

        var errorCalled = false
        Result.Error(IllegalStateException("x")).onSuccess { errorCalled = true }
        assertFalse(errorCalled)
    }

    @Test
    fun `onError 仅在 Error 时回调并返回自身`() {
        val ex = IllegalStateException("boom")
        var captured: Throwable? = null
        val result: Result<Int> = Result.Error(ex)
        assertSame(result, result.onError { captured = it })
        assertSame(ex, captured)

        var called = false
        Result.Success(1).onError { called = true }
        assertFalse(called)
    }

    @Test
    fun `onLoading 仅在 Loading 时回调并返回自身`() {
        var called = false
        val result: Result<Int> = Result.Loading
        assertSame(result, result.onLoading { called = true })
        assertTrue(called)

        var successCalled = false
        Result.Success(1).onLoading { successCalled = true }
        assertFalse(successCalled)
    }

    @Test
    fun `runCatching 成功包装结果`() {
        val result = Result.runCatching { 42 }
        assertEquals(42, result.getOrNull())
    }

    @Test
    fun `runCatching 异常包装为 Error`() {
        val ex = IllegalStateException("boom")
        val result = Result.runCatching<Int> { throw ex }
        assertSame(ex, result.exceptionOrNull())
    }

    @Test
    fun `flatMap 链式转换`() {
        val result = Result.Success(2).flatMap { Result.Success(it * 10) }
        assertEquals(20, result.getOrNull())

        val ex = IllegalStateException("x")
        val error: Result<Int> = Result.Error(ex)
        val failed = error.flatMap { Result.Success(it * 10) }
        assertSame(ex, failed.exceptionOrNull())

        val loading = Result.Loading.flatMap { Result.Success(it) }
        assertEquals(Result.Loading, loading)
    }

    @Test
    fun `getOrDefault 返回默认值`() {
        assertEquals(1, Result.Success(1).getOrDefault(5))
        val error: Result<Int> = Result.Error(IllegalStateException("x"))
        assertEquals(5, error.getOrDefault(5))
        assertEquals(5, Result.Loading.getOrDefault(5))
    }

    @Test
    fun `getOrThrow Success 返回数据`() {
        assertEquals(1, Result.Success(1).getOrThrow())
    }

    @Test
    fun `getOrThrow Error 抛出原始异常`() {
        val ex = IllegalStateException("boom")
        val error: Result<Int> = Result.Error(ex)
        try {
            error.getOrThrow()
            fail("应当抛出原始异常")
        } catch (e: IllegalStateException) {
            assertSame(ex, e)
        }
    }

    @Test
    fun `getOrThrow Loading 抛出 IllegalStateException`() {
        try {
            Result.Loading.getOrThrow()
            fail("应当抛出 IllegalStateException")
        } catch (e: IllegalStateException) {
            assertEquals("Result is still loading", e.message)
        }
    }
}
