package com.todoapp.data.remote

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class RetrofitClientRefreshTest {

    private lateinit var server: MockWebServer
    private lateinit var context: Context

    @Before
    fun setup() {
        server = MockWebServer()
        server.start()
        RetrofitClient.setBaseUrl(server.url("/").toString())
        context = ApplicationProvider.getApplicationContext()
        RetrofitClient.clearToken(context)
    }

    @After
    fun tearDown() {
        server.shutdown()
        RetrofitClient.clearToken(context)
    }

    @Test
    fun `刷新返回200时保存新令牌并返回 Success`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("{\"access_token\":\"t2\"}"))

        val outcome = RetrofitClient.tryRefreshToken(context)

        assertEquals(RetrofitClient.RefreshOutcome.Success, outcome)
        assertEquals("t2", RetrofitClient.getAccessToken(context))
    }

    @Test
    fun `刷新返回401时返回 InvalidSession`() {
        server.enqueue(MockResponse().setResponseCode(401))

        val outcome = RetrofitClient.tryRefreshToken(context)

        assertEquals(RetrofitClient.RefreshOutcome.InvalidSession, outcome)
    }

    @Test
    fun `刷新返回403时返回 InvalidSession`() {
        server.enqueue(MockResponse().setResponseCode(403))

        val outcome = RetrofitClient.tryRefreshToken(context)

        assertEquals(RetrofitClient.RefreshOutcome.InvalidSession, outcome)
    }

    @Test
    fun `刷新返回500时返回 TransientError`() {
        server.enqueue(MockResponse().setResponseCode(500))

        val outcome = RetrofitClient.tryRefreshToken(context)

        assertEquals(RetrofitClient.RefreshOutcome.TransientError, outcome)
    }

    @Test
    fun `刷新连接失败时返回 TransientError`() {
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START))

        val outcome = RetrofitClient.tryRefreshToken(context)

        assertEquals(RetrofitClient.RefreshOutcome.TransientError, outcome)
    }
}
