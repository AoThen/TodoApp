package com.todoapp.data.remote

import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class AuthInterceptorTest {

    private lateinit var server: MockWebServer
    private val lock = Any()

    private class FakeTokenStore(initial: String? = null) {
        var token: String? = initial
        val getToken: () -> String? = { token }
        val clearToken: () -> Unit = { token = null }
    }

    @Before
    fun setup() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun buildInterceptor(
        store: FakeTokenStore,
        refreshOutcome: RetrofitClient.RefreshOutcome = RetrofitClient.RefreshOutcome.Success,
        onRefresh: () -> Unit = {},
        onExpired: () -> Unit = {}
    ): AuthInterceptor {
        return AuthInterceptor(
            getToken = store.getToken,
            clearToken = store.clearToken,
            tryRefresh = { onRefresh(); refreshOutcome },
            onSessionExpired = onExpired,
            lock = lock
        )
    }

    private fun execute(interceptor: AuthInterceptor, path: String, block: (Response) -> Unit) {
        val client = OkHttpClient.Builder().addInterceptor(interceptor).build()
        client.newCall(Request.Builder().url(server.url(path)).build()).execute().use(block)
    }

    @Test
    fun `非401响应直接透传且不触发刷新`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("{\"ok\":true}"))
        var refreshCalls = 0
        val store = FakeTokenStore("old-token")

        execute(buildInterceptor(store, onRefresh = { refreshCalls++ }), "/tasks") { response ->
            assertEquals(200, response.code)
            assertEquals(0, refreshCalls)
            val request = server.takeRequest()
            assertEquals("Bearer old-token", request.getHeader("Authorization"))
        }
    }

    @Test
    fun `登录接口401直接透传且不触发刷新`() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{\"msg\":\"bad credentials\"}"))
        var refreshCalls = 0
        val store = FakeTokenStore("old-token")

        execute(buildInterceptor(store, onRefresh = { refreshCalls++ }), "/api/v1/auth/login") { response ->
            assertEquals(401, response.code)
            assertEquals("{\"msg\":\"bad credentials\"}", response.body?.string())
            assertEquals(0, refreshCalls)
        }
    }

    @Test
    fun `401且刷新成功时携带新令牌重试`() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{}"))
        server.enqueue(MockResponse().setResponseCode(200).setBody("{\"ok\":true}"))
        var refreshCalls = 0
        val store = FakeTokenStore("old-token")

        execute(
            buildInterceptor(store, onRefresh = {
                refreshCalls++
                store.token = "new-token"
            }),
            "/tasks"
        ) { response ->
            assertEquals(200, response.code)
            assertEquals(1, refreshCalls)
            val first = server.takeRequest()
            assertEquals("Bearer old-token", first.getHeader("Authorization"))
            val retry = server.takeRequest()
            assertEquals("Bearer new-token", retry.getHeader("Authorization"))
            assertEquals("application/json", retry.getHeader("Content-Type"))
            assertEquals("application/json", retry.getHeader("Accept"))
        }
    }

    @Test
    fun `拿锁后令牌已被并发刷新时跳过刷新直接重试`() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{}"))
        server.enqueue(MockResponse().setResponseCode(200).setBody("{\"ok\":true}"))
        var refreshCalls = 0
        var getCalls = 0
        val getToken: () -> String? = { if (++getCalls <= 1) "old-token" else "newer-token" }

        val interceptor = AuthInterceptor(
            getToken = getToken,
            clearToken = {},
            tryRefresh = { refreshCalls++; RetrofitClient.RefreshOutcome.Success },
            onSessionExpired = {},
            lock = lock
        )

        execute(interceptor, "/tasks") { response ->
            assertEquals(200, response.code)
            assertEquals(0, refreshCalls)
            server.takeRequest()
            val retry = server.takeRequest()
            assertEquals("Bearer newer-token", retry.getHeader("Authorization"))
        }
    }

    @Test
    fun `刷新被拒时清除令牌并广播会话失效`() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{}"))
        val store = FakeTokenStore("old-token")
        var expiredCalled = false

        execute(
            buildInterceptor(
                store,
                refreshOutcome = RetrofitClient.RefreshOutcome.InvalidSession,
                onExpired = { expiredCalled = true }
            ),
            "/tasks"
        ) { response ->
            assertEquals(401, response.code)
            assertNull(store.token)
            assertTrue(expiredCalled)
        }
    }

    @Test
    fun `刷新瞬态失败时保留原始401响应`() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("{\"msg\":\"expired\"}"))
        val store = FakeTokenStore("old-token")
        var cleared = false

        execute(
            AuthInterceptor(
                getToken = store.getToken,
                clearToken = { cleared = true },
                tryRefresh = { RetrofitClient.RefreshOutcome.TransientError },
                onSessionExpired = {},
                lock = lock
            ),
            "/tasks"
        ) { response ->
            assertEquals(401, response.code)
            assertEquals("{\"msg\":\"expired\"}", response.body?.string())
            assertFalse(cleared)
            assertEquals("old-token", store.token)
        }
    }

    @Test
    fun `无令牌时请求不带Authorization头`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("{}"))

        execute(buildInterceptor(FakeTokenStore(null)), "/tasks") { response ->
            assertEquals(200, response.code)
            val request = server.takeRequest()
            assertNull(request.getHeader("Authorization"))
        }
    }

    @Test
    fun `并发双401只触发一次刷新`() {
        val bothInitialArrived = CountDownLatch(2)
        val requestCount = AtomicInteger(0)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (requestCount.incrementAndGet() <= 2) {
                    bothInitialArrived.countDown()
                    // 等两个初始请求都到达后才返回 401，保证两个请求都进入刷新分支；
                    // 超时直接失败，避免 dispatch 线程无限挂起
                    if (!bothInitialArrived.await(5, TimeUnit.SECONDS)) {
                        throw AssertionError("两个初始请求未能在超时前到达")
                    }
                    return MockResponse().setResponseCode(401).setBody("{}")
                }
                return MockResponse().setResponseCode(200).setBody("{\"ok\":true}")
            }
        }
        var refreshCalls = 0
        val store = FakeTokenStore("old-token")
        val interceptor = buildInterceptor(store, onRefresh = {
            refreshCalls++
            store.token = "new-token"
        })
        val client = OkHttpClient.Builder().addInterceptor(interceptor).build()
        val request = Request.Builder().url(server.url("/tasks")).build()
        val executor = Executors.newFixedThreadPool(2)
        try {
            val first = executor.submit { client.newCall(request).execute() }
            val second = executor.submit { client.newCall(request).execute() }
            first.get(5, TimeUnit.SECONDS).use { assertEquals(200, it.code) }
            second.get(5, TimeUnit.SECONDS).use { assertEquals(200, it.code) }
        } finally {
            executor.shutdownNow()
        }

        assertEquals(1, refreshCalls)
        assertEquals("new-token", store.token)
    }
}
