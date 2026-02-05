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
            checkCameraPermissionAndScan()
        }
        
        binding.btnManualPair.setOnClickListener {
            showManualPairDialog()
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
                        navigateToLogin()
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
        if (result.resultCode == android.app.Activity.RESULT_OK) {
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