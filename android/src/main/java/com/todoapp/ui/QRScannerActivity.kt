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
import androidx.camera.view.PreviewView
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

                // 校验服务端地址：release 强制 https，防止恶意二维码把后续流量（含配对密钥）导向任意主机
                if (!isAcceptableServerUrl(pairingData.server)) {
                    showInvalidQRCodeDialog()
                    return@runOnUiThread
                }

                // 验证配对凭据格式
                if (!AesGcmManager.isValidKey(pairingData.key)) {
                    showInvalidKeyDialog()
                    return@runOnUiThread
                }

                // 仅接受 v2 及以上的二维码（v1 会通过二维码明文携带长期密钥，已废弃）
                if (pairingData.version < 2) {
                    showOutdatedQRCodeDialog()
                    return@runOnUiThread
                }

                // 检查过期时间
                val currentTime = System.currentTimeMillis()
                val expiresAt = pairingData.expires * 1000
                if (currentTime > expiresAt) {
                    showExpiredDialog()
                    return@runOnUiThread
                }

                // 二维码仅含一次性配对码；实际配对与设备密钥领取交由 PairingViewModel 完成
                setResult(RESULT_SUCCESS, Intent().apply { putExtra(EXTRA_QR_RESULT, rawValue) })
                finish()
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

    private fun isAcceptableServerUrl(url: String): Boolean {
        val normalized = url.trim()
        if (normalized.startsWith("https://")) return true
        // debug 构建允许明文 http（本地/模拟器调试）；release 强制 https
        return com.todoapp.BuildConfig.DEBUG && normalized.startsWith("http://")
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

    private fun showOutdatedQRCodeDialog() {
        AlertDialog.Builder(this)
            .setTitle("二维码版本过旧")
            .setMessage("此二维码缺少通道加密密钥，请在网页端重新生成配对二维码后再试。")
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
    }
}
