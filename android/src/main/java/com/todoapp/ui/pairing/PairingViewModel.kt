package com.todoapp.ui.pairing

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import com.google.gson.annotations.SerializedName
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.data.crypto.AesGcmManager
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.InitPairingRequest
import com.todoapp.data.remote.RetrofitClient
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
    @SerializedName("expires") val expires: String?,
    // 通道加密密钥：与一次性配对凭据 key 不同，旧版二维码无此字段
    @SerializedName("encryption_key") val encryptionKey: String? = null
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

                var channelKey = pairingData.encryptionKey
                var serverUrl = pairingData.server

                if (pairingData.key.isNullOrEmpty()) {
                    // 无配对凭据时由 App 自行发起配对，直接领取通道密钥
                    val response = RetrofitClient.getApiService(context)
                        .initPairing(InitPairingRequest("android", null))

                    val initBody = response.body()
                    if (response.isSuccessful && initBody != null) {
                        channelKey = initBody.encryptionKey
                        serverUrl = initBody.serverUrl
                    } else {
                        _pairingState.value = PairingState.Error(
                            context.getString(R.string.pairing_failed) + ": 无法获取配对密钥"
                        )
                        return@launch
                    }
                }

                // 二维码必须携带通道密钥（旧版二维码无此字段），否则加密通道无法建立
                val resolvedChannelKey = channelKey
                if (resolvedChannelKey.isNullOrEmpty() || !AesGcmManager.isValidKey(resolvedChannelKey)) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_invalid_qr)
                    )
                    return@launch
                }

                // 保存通道加密密钥（而非一次性配对凭据），加密通道才能真正建立
                KeyStorage.saveKey(context, resolvedChannelKey, serverUrl)
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

    private fun parsePairingData(qrData: String): PairingData? {
        // 非法 JSON 或字面量 "null" 会得到 null，由调用方按无效二维码处理
        return Gson().fromJson(qrData, PairingData::class.java)
    }

    private fun isValidPairingData(data: PairingData): Boolean {
        if (data.type != "todoapp-pairing" || data.v != 1) {
            return false
        }

        if (data.server.isBlank()) {
            return false
        }

        // 服务端地址校验：release 强制 https，防止恶意二维码把流量导向任意主机；
        // debug 构建放行 http 以便本地/模拟器调试
        val serverOk = data.server.startsWith("https://") ||
            (com.todoapp.BuildConfig.DEBUG && data.server.startsWith("http://"))
        if (!serverOk) {
            return false
        }

        if (!data.key.isNullOrEmpty()) {
            return data.key.matches(Regex("[0-9a-fA-F]{64}"))
        }

        return true
    }
}
