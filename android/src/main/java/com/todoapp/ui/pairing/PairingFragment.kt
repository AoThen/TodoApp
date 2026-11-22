package com.todoapp.ui.pairing

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import androidx.fragment.app.Fragment
import androidx.fragment.app.viewModels
import androidx.lifecycle.lifecycleScope
import androidx.navigation.fragment.findNavController
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.todoapp.R
import com.todoapp.TodoApp
import com.todoapp.databinding.FragmentPairingBinding
import com.todoapp.ui.QRScannerActivity
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.launch

@AndroidEntryPoint
class PairingFragment : Fragment() {

    private var _binding: FragmentPairingBinding? = null
    private val binding get() = _binding!!
    
    private val viewModel: PairingViewModel by viewModels()
    
    private val cameraPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { isGranted ->
        if (isGranted) {
            startQRScanner()
        } else {
            showCameraPermissionDialog()
        }
    }

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentPairingBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        setupClickListeners()
        setupObservers()
    }

    private fun setupClickListeners() {
        binding.btnScanQR.setOnClickListener {
            ensureLoggedInThen { checkCameraPermissionAndScan() }
        }
        
        binding.btnManualPair.setOnClickListener {
            ensureLoggedInThen { showManualPairDialog() }
        }
    }

    /**
     * 设备配对接口需要登录态（服务端鉴权路由），未登录时先引导登录
     */
    private fun ensureLoggedInThen(action: () -> Unit) {
        val app = requireActivity().application as TodoApp
        if (app.isUserLoggedIn()) {
            action()
        } else {
            Toast.makeText(requireContext(), R.string.pairing_login_required_message, Toast.LENGTH_LONG).show()
            findNavController().navigate(R.id.loginFragment)
        }
    }

    private fun setupObservers() {
        viewLifecycleOwner.lifecycleScope.launch {
            viewModel.pairingState.collect { state ->
                when (state) {
                    is PairingState.Idle -> {
                        binding.progressBar.visibility = View.GONE
                        binding.btnScanQR.isEnabled = true
                        binding.btnManualPair.isEnabled = true
                    }
                    is PairingState.Loading -> {
                        binding.progressBar.visibility = View.VISIBLE
                        binding.btnScanQR.isEnabled = false
                        binding.btnManualPair.isEnabled = false
                    }
                    is PairingState.Success -> {
                        binding.progressBar.visibility = View.GONE
                        Toast.makeText(requireContext(), R.string.pairing_success, Toast.LENGTH_SHORT).show()
                        // 配对成功后接通通知实时通道（未登录时内部会安全跳过）
                        com.todoapp.data.notify.NotificationSocketController.start(requireContext())
                        val app = requireActivity().application as TodoApp
                        if (app.isUserLoggedIn()) {
                            // 已登录：配对完成后直接进入主界面
                            findNavController().navigate(R.id.action_pairingFragment_to_taskListFragment)
                        } else {
                            navigateToLogin()
                        }
                    }
                    is PairingState.Error -> {
                        binding.progressBar.visibility = View.GONE
                        binding.btnScanQR.isEnabled = true
                        binding.btnManualPair.isEnabled = true
                        Toast.makeText(requireContext(), state.message, Toast.LENGTH_LONG).show()
                    }
                }
            }
        }
    }

    private fun checkCameraPermissionAndScan() {
        when {
            ContextCompat.checkSelfPermission(
                requireContext(),
                Manifest.permission.CAMERA
            ) == PackageManager.PERMISSION_GRANTED -> {
                startQRScanner()
            }
            shouldShowRequestPermissionRationale(Manifest.permission.CAMERA) -> {
                showCameraPermissionDialog()
            }
            else -> {
                cameraPermissionLauncher.launch(Manifest.permission.CAMERA)
            }
        }
    }

    private fun startQRScanner() {
        val intent = Intent(requireContext(), QRScannerActivity::class.java)
        qrScannerLauncher.launch(intent)
    }

    private val qrScannerLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        // 扫码页以自定义 RESULT_SUCCESS(1001) 表示配对码获取成功
        if (result.resultCode == QRScannerActivity.RESULT_SUCCESS) {
            val qrData = result.data?.getStringExtra(QRScannerActivity.EXTRA_QR_RESULT)
            if (!qrData.isNullOrEmpty()) {
                viewModel.pairDevice(qrData, requireContext())
            } else {
                Toast.makeText(requireContext(), R.string.pairing_invalid_qr, Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showCameraPermissionDialog() {
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(R.string.pairing_camera_permission_required)
            .setMessage(getString(R.string.pairing_camera_permission_required))
            .setPositiveButton(R.string.ok) { _, _ ->
                cameraPermissionLauncher.launch(Manifest.permission.CAMERA)
            }
            .setNegativeButton(R.string.cancel, null)
            .show()
    }

    private fun showManualPairDialog() {
        val view = LayoutInflater.from(requireContext()).inflate(R.layout.dialog_manual_pair, null)
        val editText = view.findViewById<com.google.android.material.textfield.TextInputEditText>(R.id.editPairingCode)
        
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(R.string.pairing_manual_pair)
            .setView(view)
            .setPositiveButton(R.string.ok) { _, _ ->
                val pairingCode = editText.text.toString().trim()
                if (pairingCode.isNotEmpty()) {
                    viewModel.pairDevice(pairingCode, requireContext())
                } else {
                    Toast.makeText(requireContext(), R.string.pairing_invalid_qr, Toast.LENGTH_SHORT).show()
                }
            }
            .setNegativeButton(R.string.cancel, null)
            .show()
    }

    private fun navigateToLogin() {
        findNavController().navigate(R.id.action_pairingFragment_to_loginFragment)
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}