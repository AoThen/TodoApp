package com.todoapp.ui.pairing

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import com.todoapp.R
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.RetrofitClient
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

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

@HiltViewModel
class PairingViewModel @Inject constructor() : ViewModel() {

    private var _pairingState = MutableStateFlow<PairingState>(PairingState.Idle)
    val pairingState: StateFlow<PairingState> = _pairingState.asStateFlow()

    fun pairDevice(qrData: String, context: android.content.Context) {
        viewModelScope.launch {
            _pairingState.value = PairingState.Loading

            try {
                val pairingData = parsePairingData(qrData)

                if (!isValidPairingData(pairingData)) {
                    _pairingState.value = PairingState.Error(
                        context.getString(R.string.pairing_failed)
                    )
                    return@launch
                }

                KeyStorage.saveKey(context, pairingData.key, pairingData.server)
                RetrofitClient.setBaseUrl(pairingData.server)

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
