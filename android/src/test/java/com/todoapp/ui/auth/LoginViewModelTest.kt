package com.todoapp.ui.auth

import androidx.test.core.app.ApplicationProvider
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.LoginRequest
import com.todoapp.data.remote.LoginResponse
import com.todoapp.data.remote.RetrofitClient
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import io.mockk.mockkObject
import io.mockk.unmockkAll
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.setMain
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class LoginViewModelTest {

    private lateinit var viewModel: LoginViewModel
    private lateinit var apiService: ApiService

    private val loginResponse = mockk<LoginResponse> {
        every { accessToken } returns "access-token-x"
        every { refreshToken } returns "refresh-token-x"
    }

    @Before
    fun setup() {
        Dispatchers.setMain(UnconfinedTestDispatcher())
        apiService = mockk()
        mockkObject(RetrofitClient)
        every { RetrofitClient.saveAccessToken(any(), any()) } returns Unit
        every { RetrofitClient.saveRefreshToken(any(), any()) } returns Unit
        viewModel = LoginViewModel(ApplicationProvider.getApplicationContext(), apiService)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
        unmockkAll()
    }

    @Test
    fun `初始状态为Idle且表单有效标志为false`() {
        assertTrue(viewModel.loginState.value is LoginState.Idle)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `validateEmail 空白邮箱提示必填`() {
        viewModel.validateEmail("  ")
        val state = viewModel.formState.value
        assertNotNull(state.emailError)
        assertFalse(state.isFormValid)
    }

    @Test
    fun `validateEmail 非法格式提示无效`() {
        viewModel.validateEmail("not-an-email")
        assertNotNull(viewModel.formState.value.emailError)
    }

    @Test
    fun `validateEmail 合法邮箱无错误`() {
        viewModel.validateEmail("user@example.com")
        assertNull(viewModel.formState.value.emailError)
    }

    @Test
    fun `validatePassword 短于6位提示错误`() {
        viewModel.validatePassword("123")
        assertNotNull(viewModel.formState.value.passwordError)
        assertFalse(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `validatePassword 合法密码无错误且表单有效`() {
        viewModel.validateEmail("user@example.com")
        viewModel.validatePassword("password123")
        assertNull(viewModel.formState.value.passwordError)
        assertTrue(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `validateForm 一次性校验组合`() {
        viewModel.validateForm("bad-email", "123")
        assertNotNull(viewModel.formState.value.emailError)
        assertNotNull(viewModel.formState.value.passwordError)

        viewModel.validateForm("user@example.com", "password123")
        assertNull(viewModel.formState.value.emailError)
        assertNull(viewModel.formState.value.passwordError)
        assertTrue(viewModel.formState.value.isFormValid)
    }

    @Test
    fun `login 成功后进入Success并保存双token`() {
        coEvery { apiService.login(any<LoginRequest>()) } returns Response.success(loginResponse)

        viewModel.login("user@example.com", "password123")

        val state = viewModel.loginState.value
        assertTrue(state is LoginState.Success)
        assertEquals("access-token-x", (state as LoginState.Success).token)
        coVerify { apiService.login(any<LoginRequest>()) }
        io.mockk.verify { RetrofitClient.saveAccessToken(any(), "access-token-x") }
        io.mockk.verify { RetrofitClient.saveRefreshToken(any(), "refresh-token-x") }
    }

    @Test
    fun `login HTTP错误时进入Error并解析message`() {
        val errorBody = """{"message":"邮箱或密码错误"}""".toResponseBody("application/json".toMediaType())
        coEvery { apiService.login(any<LoginRequest>()) } returns Response.error(401, errorBody)

        viewModel.login("user@example.com", "wrong-password")

        val state = viewModel.loginState.value
        assertTrue(state is LoginState.Error)
        assertEquals("邮箱或密码错误", (state as LoginState.Error).message)
    }

    @Test
    fun `login 网络异常时进入Error`() {
        coEvery { apiService.login(any<LoginRequest>()) } throws java.net.ConnectException("network down")

        viewModel.login("user@example.com", "password123")

        val state = viewModel.loginState.value
        assertTrue(state is LoginState.Error)
        assertNotNull((state as LoginState.Error).message)
    }
}
