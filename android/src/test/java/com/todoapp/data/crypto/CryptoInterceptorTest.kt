package com.todoapp.data.crypto

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import io.mockk.CapturingSlot
import io.mockk.every
import io.mockk.mockk
import io.mockk.mockkObject
import io.mockk.unmockkAll
import io.mockk.verify
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import okio.Buffer

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class CryptoInterceptorTest {

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

    private lateinit var interceptor: CryptoInterceptor

    @Before
    fun setUp() {
        mockkObject(KeyStorage)
        every { KeyStorage.getKey(any()) } returns keyHex
        interceptor = CryptoInterceptor.getInstance(context)
        interceptor.refreshKey()
    }

    @After
    fun tearDown() {
        unmockkAll()
    }

    private fun buildResponse(request: Request, body: String, encrypted: Boolean): Response {
        val builder = Response.Builder()
            .protocol(Protocol.HTTP_1_1)
            .code(200)
            .message("OK")
            .request(request)
        if (encrypted) builder.header("X-Encrypted", "true")
        return builder.body(body.toResponseBody("application/json".toMediaType())).build()
    }

    private fun mockChain(
        request: Request,
        responseFactory: (Request) -> Response = { buildResponse(it, "{}", encrypted = false) }
    ): Interceptor.Chain {
        val chain = mockk<Interceptor.Chain>()
        every { chain.request() } returns request
        every { chain.proceed(any()) } answers { responseFactory(firstArg()) }
        return chain
    }

    /** 执行拦截并返回实际传给 chain.proceed 的请求 */
    private fun proceedRequest(chain: Interceptor.Chain): Request {
        interceptor.intercept(chain)
        val slot = CapturingSlot<Request>()
        verify { chain.proceed(capture(slot)) }
        return slot.captured
    }

    private fun jsonRequest(url: String, method: String, body: String? = null): Request {
        val builder = Request.Builder().url(url)
        when (method) {
            "GET" -> builder.get()
            "DELETE" -> builder.delete()
            else -> builder.method(
                method,
                (body ?: "").toRequestBody("application/json".toMediaType())
            )
        }
        return builder.build()
    }

    @Test
    fun `无密钥时请求透传不加密`() {
        every { KeyStorage.getKey(any()) } returns null
        interceptor.refreshKey()

        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", """{"title":"t"}""")
        val sent = proceedRequest(mockChain(request))

        assertNull(sent.header("X-Encrypted"))
        assertNull(sent.header("X-Accept-Encrypted"))
    }

    @Test
    fun `POST tasks 请求体被加密并携带加密头`() {
        val original = """{"title":"写测试","priority":"high"}"""
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", original)
        val sent = proceedRequest(mockChain(request))

        assertEquals("true", sent.header("X-Encrypted"))
        assertEquals("true", sent.header("X-Accept-Encrypted"))
        assertEquals("application/octet-stream", sent.body?.contentType()?.toString())

        val sentBytes = Buffer().also { sent.body!!.writeTo(it) }.readByteArray()
        val decrypted = String(AesGcmManager.decryptBytes(sentBytes, keyHex), Charsets.UTF_8)
        assertEquals(original, decrypted)
    }

    @Test
    fun `GET tasks 列表不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `单个任务 GET 不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks/42", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `单个任务 DELETE 不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks/42", "DELETE")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `auth 路径不加密`() {
        val request = jsonRequest(
            "http://10.0.2.2:8080/api/v1/auth/login",
            "POST",
            """{"email":"a@b.c","password":"p"}"""
        )
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `admin 路径不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/admin/users", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `health 路径不加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/health", "GET")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }

    @Test
    fun `sync 路径需要加密`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val sent = proceedRequest(mockChain(request))
        assertEquals("true", sent.header("X-Encrypted"))
    }

    @Test
    fun `已带 X-Encrypted 头的请求不重复加密`() {
        val request = Request.Builder()
            .url("http://10.0.2.2:8080/api/v1/tasks")
            .header("X-Encrypted", "true")
            .post("""{"title":"t"}""".toRequestBody("application/json".toMediaType()))
            .build()
        val sent = proceedRequest(mockChain(request))

        // 不应再次覆盖加密流程：body 仍是原始 JSON 而非密文
        val sentBytes = Buffer().also { sent.body!!.writeTo(it) }.readByteArray()
        assertEquals("""{"title":"t"}""", String(sentBytes, Charsets.UTF_8))
    }

    @Test
    fun `加密响应体被自动解密为明文 JSON`() {
        val secret = """{"success":true,"data":{"id":1}}"""
        val encryptedBody = AesGcmManager.encrypt(secret, keyHex)

        // POST /sync 走加密请求路径，服务端返回加密响应时应被自动解密
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val chain = mockChain(request) { buildResponse(it, encryptedBody, encrypted = true) }
        val response = interceptor.intercept(chain)

        assertEquals("application/json", response.body?.contentType()?.toString())
        assertEquals(secret, response.body?.string())
        assertNull(response.header("X-Encrypted"))
    }

    @Test
    fun `未加密响应体原样返回`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/sync", "POST", """{"deltas":[]}""")
        val chain = mockChain(request) { buildResponse(it, """{"ok":true}""", encrypted = false) }
        val response = interceptor.intercept(chain)

        assertEquals("""{"ok":true}""", response.body?.string())
    }

    @Test
    fun `空请求体不触发加密直接透传`() {
        val request = jsonRequest("http://10.0.2.2:8080/api/v1/tasks", "POST", "")
        val sent = proceedRequest(mockChain(request))
        assertNull(sent.header("X-Encrypted"))
    }
}
