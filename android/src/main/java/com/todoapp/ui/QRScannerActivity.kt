package com.todoapp.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.util.Log
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScanner
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import com.todoapp.R
import com.todoapp.data.crypto.AesGcmManager
import com.todoapp.data.crypto.KeyStorage
import com.todoapp.data.remote.PairingRequest
import com.todoapp.data.remote.PairingResponse
import com.todoapp.data.remote.RetrofitClient
import androidx.camera.view.PreviewView
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import org.json.JSONObject
import retrofit2.Response
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

class QRScannerActivity : AppCompatActivity() {

    private lateinit var cameraExecutor: ExecutorService
    private lateinit var barcodeScanner: BarcodeScanner
    private var isScanning = true

    private val requestPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { isGranted ->
        if (isGranted) {
            startCamera()
        } else {
            showPermissionDeniedDialog()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_qr_scanner)

        cameraExecutor = Executors.newSingleThreadExecutor()

        val options = BarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .build()
        barcodeScanner = BarcodeScanning.getClient(options)

        checkCameraPermission()
    }

    private fun checkCameraPermission() {
        when {
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.CAMERA
            ) == PackageManager.PERMISSION_GRANTED -> {
                startCamera()
            }
            else -> {
                requestPermissionLauncher.launch(Manifest.permission.CAMERA)
            }
        }
    }

    private fun startCamera() {
        val cameraProviderFuture = ProcessCameraProvider.getInstance(this)

        cameraProviderFuture.addListener({
            val cameraProvider = cameraProviderFuture.get()

            val preview = Preview.Builder()
                .build()
                .also {
                    val previewView = findViewById<PreviewView>(R.id.previewView)
                    it.setSurfaceProvider(previewView.getSurfaceProvider())
                }

            val imageAnalyzer = ImageAnalysis.Builder()
                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                .build()
                .also {
                    it.setAnalyzer(cameraExecutor, QRCodeAnalyzer { barcodeValue ->
                        if (isScanning) {
                            isScanning = false
                            handleScannedCode(barcodeValue)
                        }
                    })
                }

            val cameraSelector = CameraSelector.DEFAULT_BACK_CAMERA

            try {
                cameraProvider.unbindAll()
                cameraProvider.bindToLifecycle(
                    this,
                    cameraSelector,
                    preview,
                    imageAnalyzer
                )
            } catch (e: Exception) {
                Log.e(TAG, "Camera binding failed", e)
                Toast.makeText(this, "Camera initialization failed", Toast.LENGTH_SHORT).show()
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun handleScannedCode(rawValue: String) {
        runOnUiThread {
            try {
                val pairingData = parsePairingData(rawValue)

                if (pairingData == null) {
                    showInvalidQRCodeDialog()
                    return@runOnUiThread
                }

                // 验证密钥格式
                if (!AesGcmManager.isValidKey(pairingData.key)) {
                    showInvalidKeyDialog()
                    return@runOnUiThread
                }

                // 检查过期时间
                val currentTime = System.currentTimeMillis()
                val expiresAt = pairingData.expires * 1000
                if (currentTime > expiresAt) {
                    showExpiredDialog()
                    return@runOnUiThread
                }

                // 显示加载中
                showLoading()

                // 调用服务器验证配对（Token由AuthInterceptor自动添加）
                GlobalScope.launch(Dispatchers.IO) {
                    try {
                        val deviceId = getDeviceID()
                        val response: Response<PairingResponse> = RetrofitClient.getApiService(this@QRScannerActivity)
                            .pairDevice(
                                PairingRequest(
                                    key = pairingData.key,
                                    deviceType = "android",
                                    deviceId = deviceId!!
                                )
                            )

                        runOnUiThread {
                            dismissLoading()

                            if (response.isSuccessful && response.body() != null) {
                                // 服务器验证成功
                                KeyStorage.init(this@QRScannerActivity)
                                KeyStorage.saveKey(this@QRScannerActivity, pairingData.key, pairingData.server)

                                // 保存设备信息
                                val prefs = getSharedPreferences("TodoAppPrefs", MODE_PRIVATE)
                                prefs.edit()
                                    .putString("device_id", deviceId)
                                    .putString("server_url", pairingData.server)
                                    .apply()

                                Log.d(TAG, "设备配对成功: $deviceId")
                                showSuccessDialog(pairingData.server)
                            } else {
                                val errorMsg = response.errorBody()?.string() ?: "配对验证失败"
                                Log.e(TAG, "配对失败: $errorMsg")
                                showErrorDialog(getErrorMessage(errorMsg))
                            }
                        }
                    } catch (e: Exception) {
                        runOnUiThread {
                            dismissLoading()
                            Log.e(TAG, "配对异常", e)
                            showErrorDialog("配对验证失败: ${e.message}")
                        }
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "Error processing QR code", e)
                showErrorDialog("Error: ${e.message}")
            }
        }
    }

    private fun parsePairingData(rawValue: String): PairingData? {
        return try {
            val json = org.json.JSONObject(rawValue)

            if (json.optString("type") != "todoapp-pairing") {
                return null
            }

            PairingData(
                version = json.optInt("v", 1),
                type = json.optString("type"),
                key = json.optString("key"),
                server = json.optString("server"),
                expires = json.optLong("expires", 0)
            )
        } catch (e: Exception) {
            Log.e(TAG, "Failed to parse pairing data", e)
            null
        }
    }

    private fun getDeviceID(): String {
        val prefs = getSharedPreferences("TodoAppPrefs", MODE_PRIVATE)
        var deviceId = prefs.getString("device_id", null)
        if (deviceId.isNullOrEmpty()) {
            deviceId = android.provider.Settings.Secure.getString(
                contentResolver,
                android.provider.Settings.Secure.ANDROID_ID
            ) ?: return ""
            prefs.edit().putString("device_id", deviceId).apply()
        }
        return deviceId
    }

    private fun getErrorMessage(errorBody: String): String {
        return try {
            val json = JSONObject(errorBody)
            json.optString("error") ?: json.optString("message") ?: "配对失败"
        } catch (e: Exception) {
            "配对失败"
        }
    }

    private fun showLoading() {
        // 可以实现一个加载对话框
    }

    private fun dismissLoading() {
        // 关闭加载对话框
    }

    private fun showExpiredDialog() {
        AlertDialog.Builder(this)
            .setTitle("二维码已过期")
            .setMessage("该配对二维码已过期，请刷新后重新扫描")
            .setPositiveButton("重试") { _, _ ->
                isScanning = true
            }
            .setNegativeButton("取消") { _, _ ->
                finish()
            }
            .show()
    }

    private fun showNotLoggedInDialog() {
        AlertDialog.Builder(this)
            .setTitle("需要登录")
            .setMessage("请先登录后再进行设备配对")
            .setPositiveButton("确定") { _, _ ->
                finish()
            }
            .show()
    }

    private fun showSuccessDialog(serverUrl: String) {
        AlertDialog.Builder(this)
            .setTitle("配对成功")
            .setMessage("设备已成功配对到: $serverUrl\n\n所有通信现在将使用 AES-256-GCM 加密。")
            .setPositiveButton("确定") { _, _ ->
                val resultIntent = Intent().apply {
                    putExtra(EXTRA_QR_RESULT, "success")
                }
                setResult(RESULT_SUCCESS, resultIntent)
                finish()
            }
            .setCancelable(false)
            .show()
    }

    private fun showInvalidQRCodeDialog() {
        AlertDialog.Builder(this)
            .setTitle("无效二维码")
            .setMessage("此二维码不是有效的 TodoApp 配对二维码。")
            .setPositiveButton("重试") { _, _ ->
                isScanning = true
            }
            .setNegativeButton("取消") { _, _ ->
                finish()
            }
            .show()
    }

    private fun showInvalidKeyDialog() {
        AlertDialog.Builder(this)
            .setTitle("无效密钥")
            .setMessage("二维码中的密钥格式无效。")
            .setPositiveButton("重试") { _, _ ->
                isScanning = true
            }
            .setNegativeButton("取消") { _, _ ->
                finish()
            }
            .show()
    }

    private fun showPermissionDeniedDialog() {
        AlertDialog.Builder(this)
            .setTitle("需要相机权限")
            .setMessage("扫描二维码需要相机权限。请在设置中启用相机权限。")
            .setPositiveButton("确定") { _, _ ->
                finish()
            }
            .setCancelable(false)
            .show()
    }

    private fun showErrorDialog(message: String) {
        AlertDialog.Builder(this)
            .setTitle("错误")
            .setMessage(message)
            .setPositiveButton("确定") { _, _ ->
                finish()
            }
            .show()
    }

    override fun onDestroy() {
        super.onDestroy()
        cameraExecutor.shutdown()
        barcodeScanner.close()
    }

    private inner class QRCodeAnalyzer(
        private val onQRCodeDetected: (String) -> Unit
    ) : ImageAnalysis.Analyzer {

        @androidx.camera.core.ExperimentalGetImage
        override fun analyze(imageProxy: androidx.camera.core.ImageProxy) {
            val mediaImage = imageProxy.image
            if (mediaImage != null) {
                val image = InputImage.fromMediaImage(
                    mediaImage,
                    imageProxy.imageInfo.rotationDegrees
                )

                barcodeScanner.process(image)
                    .addOnSuccessListener { barcodes ->
                        for (barcode in barcodes) {
                            barcode.rawValue?.let { value ->
                                onQRCodeDetected(value)
                            }
                        }
                    }
                    .addOnCompleteListener {
                        imageProxy.close()
                    }
            } else {
                imageProxy.close()
            }
        }
    }

    data class PairingData(
        val version: Int,
        val type: String,
        val key: String,
        val server: String,
        val expires: Long
    )

    companion object {
        private const val TAG = "QRScannerActivity"
        const val EXTRA_QR_RESULT = "qr_result"
        const val RESULT_SUCCESS = 1001
        const val RESULT_FAILED = 1002
    }
}
