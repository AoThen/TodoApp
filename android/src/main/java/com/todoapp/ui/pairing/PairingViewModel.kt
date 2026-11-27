package com.todoapp.ui.pairing

import android.provider.Settings
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import com.google.gson.annotations.SerializedName
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.data.crypto.EcdhKeyWrap
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.InitPairingRequest
import com.todoapp.data.remote.PairingRequest
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.utils.ServerUrlValidator
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

data class PairingData(
    // release 开启混淆后 Gson 反射按字段名匹配会失败，必须显式声明序列化名
    @SerializedName("type") val type: String,
    @SerializedName("v") val v: Int,
    @SerializedName("key") val key: String?,
    @SerializedName("server") val server: String,
    @SerializedName("expires") val expires: String?
)

sealed class PairingState {
    object Idle : PairingState()
    object Loading : PairingState()
    data class Success(val message: String) : PairingState()
    data class Error(val message: String) : PairingState()
}

@HiltViewModel
class PairingViewModel @Inject constructor() : ViewModel() {

    private var _pairingState = MutableStateFlow<PairingState>(PairingState.Idle)
    val pairingState: StateFlow<PairingState> = _pairingState.asStateFlow()

    fun pairDevice(qrData: String, context: android.content.Context) {
        viewModelScope.launch {
            _pairingState.value = PairingState.Loading

            try {
                val pairingData = parsePairingData(qrData)

                if (pairingData == null || !isValidPairingData(pairingData)) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed)
                    )
                    return@launch
                }

                val deviceId = resolveDeviceId(context)
                if (deviceId.isEmpty()) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed) + ": 无法获取设备ID"
                    )
                    return@launch
                }

                var pairingKey = pairingData.key
                var serverUrl = pairingData.server

                // 无配对凭据（如自定义服务器）时，先向服务端申请一次性配对码
                if (pairingKey.isNullOrEmpty()) {
                    val initResponse = RetrofitClient.getApiService(context)
                        .initPairing(InitPairingRequest("android", deviceId))
                    val initBody = initResponse.body()
                    if (!initResponse.isSuccessful || initBody == null) {
                        _pairingState.value = PairingState.Error(
                            context.getString(R.string.pairing_failed) + ": 无法获取配对密钥"
                        )
                        return@launch
                    }
                    pairingKey = initBody.key
                    serverUrl = initBody.serverUrl.ifBlank { serverUrl }
                }

                // 生成一次性客户端密钥对，用其公钥领取本设备专属密钥
                val clientKey = EcdhKeyWrap.generateClientKeyPair()
                val pairResponse = RetrofitClient.getApiService(context).pairDevice(
                    PairingRequest(
                        key = pairingKey,
                        deviceType = "android",
                        deviceId = deviceId,
                        clientPublicKey = clientKey.publicKeySpkiB64
                    )
                )
                val pairBody = pairResponse.body()
                if (!pairResponse.isSuccessful || pairBody == null) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed)
                    )
                    return@launch
                }

                val wrappedKey = pairBody.wrappedKey
                val serverPublicKey = pairBody.serverPublicKey
                val nonce = pairBody.nonce
                if (wrappedKey.isNullOrEmpty() || serverPublicKey.isNullOrEmpty() || nonce.isNullOrEmpty()) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed) + ": 配对响应缺少密钥材料"
                    )
                    return@launch
                }

                val deviceKey = EcdhKeyWrap.unwrapDeviceKey(
                    clientPrivateKey = clientKey.privateKey,
                    clientPublicSpki = clientKey.publicKeySpki,
                    serverPublicKeyB64 = serverPublicKey,
                    wrappedKeyB64 = wrappedKey,
                    nonceB64 = nonce,
                    deviceId = deviceId,
                    pairingKey = pairingKey
                )

                if (!KeyStorage.saveKey(context, deviceKey, serverUrl)) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed) + ": 本机安全存储不可用"
                    )
                    return@launch
                }

                // 同步刷新加密拦截器的内存密钥缓存，使后续请求立即启用加密通道
                com.todoapp.data.crypto.CryptoInterceptor.getInstance(context.applicationContext).refreshKey()
                RetrofitClient.setBaseUrl(serverUrl)
                // 同步更新并持久化到 AppConfig，保证 Hilt 注入的登录/业务通道与同步通道指向同一服务器
                AppConfig.setBaseUrl(context.applicationContext, serverUrl)

                _pairingState.value = PairingState.Success(
                    context.getString(R.string.pairing_success)
                )

            } catch (e: JsonSyntaxException) {
                _pairingState.value = PairingState.Error(
                    context.getString(R.string.pairing_invalid_qr)
                )
            } catch (e: Exception) {
                _pairingState.value = PairingState.Error(
                    context.getString(R.string.pairing_failed) + ": ${e.message}"
                )
            }
        }
    }

    private fun resolveDeviceId(context: android.content.Context): String {
        val prefs = context.getSharedPreferences("TodoAppPrefs", android.content.Context.MODE_PRIVATE)
        val existing = prefs.getString("device_id", null)
        if (!existing.isNullOrEmpty()) {
            return existing
        }
        val id = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ANDROID_ID
        ) ?: return ""
        prefs.edit().putString("device_id", id).apply()
        return id
    }

    private fun parsePairingData(qrData: String): PairingData? {
        // 非法 JSON 或字面量 "null" 会得到 null，由调用方按无效二维码处理
        return Gson().fromJson(qrData, PairingData::class.java)
    }

    private fun isValidPairingData(data: PairingData): Boolean {
        if (data.type != "todoapp-pairing" || data.v < 2) {
            return false
        }

        if (data.server.isBlank()) {
            return false
        }

        // 服务端地址校验：release 强制 https，防止恶意二维码把流量导向任意主机；
        // debug 构建放行 http 以便本地/模拟器调试
        if (!ServerUrlValidator.isValidServerUrl(data.server, com.todoapp.BuildConfig.DEBUG)) {
            return false
        }

        if (!data.key.isNullOrEmpty()) {
            return data.key.matches(Regex("[0-9a-fA-F]{64}"))
        }

        return true
    }
}
