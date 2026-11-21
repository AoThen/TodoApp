package com.todoapp.ui.settings

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.fragment.app.Fragment
import androidx.navigation.fragment.findNavController
import com.todoapp.BuildConfig
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.databinding.FragmentSettingsBinding
import com.todoapp.utils.ThemeUtils
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class SettingsFragment : Fragment() {

    private var _binding: FragmentSettingsBinding? = null
    private val binding get() = _binding!!

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        _binding = FragmentSettingsBinding.inflate(inflater, container, false)
        return binding.root
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        setupDarkModeToggle()
        setupInfoRows()
    }

    private fun setupDarkModeToggle() {
        // 开关展示当前生效的外观（未设置过时反映系统深色状态）
        val systemDark = (resources.configuration.uiMode and
            android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
            android.content.res.Configuration.UI_MODE_NIGHT_YES
        binding.switchDarkMode.isChecked = when {
            ThemeUtils.isDarkModeSet(requireContext()) -> ThemeUtils.isDarkMode(requireContext())
            else -> systemDark
        }
        binding.switchDarkMode.setOnCheckedChangeListener { _, checked ->
            ThemeUtils.setDarkMode(requireContext(), checked)
        }
    }

    private fun setupInfoRows() {
        // 服务器地址（只读展示，来自构建配置或本地覆盖）
        binding.tvServerUrl.text = AppConfig.getBaseUrl()

        // 应用版本号
        binding.tvVersion.text = BuildConfig.VERSION_NAME

        // 重新配对设备
        binding.btnRePair.setOnClickListener {
            findNavController().navigate(R.id.action_settingsFragment_to_pairingFragment)
        }
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}
