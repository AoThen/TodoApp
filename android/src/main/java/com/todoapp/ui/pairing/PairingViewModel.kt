package com.todoapp.ui.pairing

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import com.todoapp.TodoApp
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.RetrofitClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class PairingData(
    val type: String,
    val v: Int,
    val key: String,
    val server: String,
    val expires: Long
)

sealed class PairingState {
    object Idle : PairingState()
    object Loading : PairingState()
    data class Success(val token: String) : PairingState()
    data class Error(val message: String) : PairingState()
}

class PairingViewModel(application: Application) : AndroidViewModel(application) {
    
    private val context = getApplication<TodoApp>().applicationContext
    private val _pairingState = MutableStateFlow<PairingState>(PairingState.Idle)
    val pairingState: StateFlow<PairingState> = _pairingState.asStateFlow()

    fun pairDevice(qrData: String) {
        viewModelScope.launch {
            _pairingState.value = PairingState.Loading
            
            try {
                // Parse QR data
                val pairingData = parsePairingData(qrData)
                
                // Validate pairing data
                if (!isValidPairingData(pairingData)) {
                    _pairingState.value = PairingState.Error("无效的配对数据")
                    return@launch
                }
                
                // Store encryption key and server URL
                KeyStorage.saveKey(context, pairingData.key, pairingData.server)
                
                // Store server URL in RetrofitClient
                RetrofitClient.setBaseUrl(pairingData.server)
                
                _pairingState.value = PairingState.Success("配对成功")
                
            } catch (e: JsonSyntaxException) {
                _pairingState.value = PairingState.Error("二维码格式错误")
            } catch (e: Exception) {
                _pairingState.value = PairingState.Error("配对失败: ${e.message}")
            }
        }
    }

    private fun parsePairingData(qrData: String): PairingData {
        return Gson().fromJson(qrData, PairingData::class.java)
    }

    private fun isValidPairingData(data: PairingData): Boolean {
        return data.type == "todoapp-pairing" &&
               data.v == 1 &&
               data.key.matches(Regex("[0-9a-fA-F]{64}")) &&
               data.server.isNotBlank() &&
               data.expires > System.currentTimeMillis()
    }
}