package com.todoapp.ui.settings

import android.net.Uri
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.view.isVisible
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import androidx.navigation.NavOptions
import androidx.navigation.fragment.findNavController
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.todoapp.BuildConfig
import com.todoapp.R
import com.todoapp.config.AppConfig
import com.todoapp.data.backup.ImportExportManager
import com.todoapp.data.notify.NotificationSocketController
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.ServerSwitchHelper
import com.todoapp.data.repository.AuthRepository
import com.todoapp.data.repository.TaskRepository
import com.todoapp.databinding.FragmentSettingsBinding
import com.todoapp.utils.ServerUrlValidator
import com.todoapp.utils.ThemeUtils
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.IOException
import javax.inject.Inject

@AndroidEntryPoint
class SettingsFragment : Fragment() {

    private var _binding: FragmentSettingsBinding? = null
    private val binding get() = _binding!!

    @Inject lateinit var authRepository: AuthRepository
    @Inject lateinit var taskRepository: TaskRepository

    private val importExportManager = ImportExportManager()

    private enum class ExportFormat { JSON, CSV }

    private val exportJsonLauncher = registerForActivityResult(
        ActivityResultContracts.CreateDocument("application/json")
    ) { uri -> if (uri != null) writeExport(uri, ExportFormat.JSON) }

    private val exportCsvLauncher = registerForActivityResult(
        ActivityResultContracts.CreateDocument("text/csv")
    ) { uri -> if (uri != null) writeExport(uri, ExportFormat.CSV) }

    private val importJsonLauncher = registerForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri -> if (uri != null) importFrom(uri) }

    // 部分文件管理器会把 .json 标为 text/plain 或 application/octet-stream，一并放行，
    // 内容合法性由解析层校验兜底
    private val importMimeTypes = arrayOf(
        "application/json", "text/plain", "application/octet-stream"
    )

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
        setupDataActions()
        setupLogout()
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

        // 修改服务器地址（切换后需重新登录并配对）
        binding.btnEditServerUrl.setOnClickListener {
            showEditServerUrlDialog()
        }

        // 应用版本号
        binding.tvVersion.text = BuildConfig.VERSION_NAME

        // 重新配对设备
        binding.btnRePair.setOnClickListener {
            findNavController().navigate(R.id.action_settingsFragment_to_pairingFragment)
        }
    }

    private fun showEditServerUrlDialog() {
        val view = LayoutInflater.from(requireContext())
            .inflate(R.layout.dialog_edit_server_url, null)
        val editText = view.findViewById<com.google.android.material.textfield.TextInputEditText>(
            R.id.editServerUrl
        )
        editText.setText(AppConfig.getBaseUrl())

        MaterialAlertDialogBuilder(requireContext())
            .setTitle(R.string.settings_edit_server_url)
            .setView(view)
            .setPositiveButton(R.string.ok) { _, _ ->
                val url = editText.text.toString().trim()
                if (!ServerUrlValidator.isValidServerUrl(url, BuildConfig.DEBUG)) {
                    Toast.makeText(requireContext(), R.string.settings_server_url_invalid, Toast.LENGTH_LONG).show()
                    return@setPositiveButton
                }
                val normalized = ServerUrlValidator.normalizeBaseUrl(url)
                if (normalized == AppConfig.getBaseUrl()) return@setPositiveButton
                showConfirmServerChangeDialog(normalized)
            }
            .setNegativeButton(R.string.cancel, null)
            .show()
    }

    private fun showConfirmServerChangeDialog(newUrl: String) {
        MaterialAlertDialogBuilder(requireContext())
            .setTitle(R.string.settings_server_url_confirm_title)
            .setMessage(R.string.settings_server_url_confirm_message)
            .setPositiveButton(R.string.ok) { _, _ -> performServerUrlChange(newUrl) }
            .setNegativeButton(R.string.cancel, null)
            .show()
    }

    /**
     * 切换服务器 = 重置会话：旧服务器的登录凭据、设备配对密钥与待同步修改全部作废，
     * 防止旧账号数据以新账号身份上传到新服务器造成串写。
     * 游客态没有登录凭据，保留其待同步队列，登录后仍可合并上传。
     */
    private fun performServerUrlChange(newUrl: String) {
        val appContext = requireContext().applicationContext
        viewLifecycleOwner.lifecycleScope.launch {
            withContext(Dispatchers.IO) {
                // 切换服务器 = 重置会话：已登录需清空待同步队列，游客态保留队列待登录后合并
                ServerSwitchHelper.resetSession(
                    appContext,
                    clearPendingDeltas = authRepository.isLoggedIn()
                )
                AppConfig.setBaseUrl(appContext, newUrl)
                RetrofitClient.setBaseUrl(newUrl)
            }
            NotificationSocketController.stop()
            Toast.makeText(requireContext(), R.string.settings_server_url_changed, Toast.LENGTH_LONG).show()
            if (AppConfig.isGuestMode(appContext)) {
                // 游客态无会话可重置：原地刷新展示地址即可
                binding.tvServerUrl.text = AppConfig.getBaseUrl()
            } else {
                // 已配对信息随切换作废，回到配对入口重新走"登录 + 配对"流程
                findNavController().navigate(
                    R.id.pairingFragment,
                    null,
                    NavOptions.Builder().setPopUpTo(0, true).build()
                )
            }
        }
    }

    private fun setupDataActions() {
        binding.btnExportJson.setOnClickListener {
            exportJsonLauncher.launch(ImportExportManager.exportFileName("json"))
        }
        binding.btnExportCsv.setOnClickListener {
            exportCsvLauncher.launch(ImportExportManager.exportFileName("csv"))
        }
        binding.btnImportJson.setOnClickListener {
            importJsonLauncher.launch(importMimeTypes)
        }
    }

    private fun setupLogout() {
        // 游客/未登录态提供登录入口；已登录则展示退出登录
        binding.btnLogin.isVisible = !authRepository.isLoggedIn()
        binding.btnLogin.setOnClickListener {
            findNavController().navigate(R.id.loginFragment)
        }

        // 防御：任何路径下未登录都不展示退出登录入口
        binding.btnLogout.isVisible = authRepository.isLoggedIn()
        binding.btnLogout.setOnClickListener {
            MaterialAlertDialogBuilder(requireContext())
                .setTitle(R.string.settings_logout_confirm_title)
                .setMessage(R.string.settings_logout_confirm_message)
                .setPositiveButton(R.string.settings_logout) { _, _ -> performLogout() }
                .setNegativeButton(android.R.string.cancel, null)
                .show()
        }
    }

    private fun performLogout() {
        viewLifecycleOwner.lifecycleScope.launch {
            // 离线或服务端登出失败时也会清除本地会话
            authRepository.logout()
            // 清空待同步队列：防止旧账号的离线修改以新账号身份上传，造成跨账号串写
            taskRepository.clearDeltaQueue()
            NotificationSocketController.stop()
            Toast.makeText(requireContext(), R.string.settings_logout_done, Toast.LENGTH_SHORT).show()
            findNavController().navigate(
                R.id.loginFragment,
                null,
                NavOptions.Builder().setPopUpTo(0, true).build()
            )
        }
    }

    private fun writeExport(uri: Uri, format: ExportFormat) {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = withContext(Dispatchers.IO) {
                try {
                    val tasks = taskRepository.getAllTasks().first()
                    val content = when (format) {
                        ExportFormat.JSON -> importExportManager.toJson(tasks)
                        ExportFormat.CSV -> importExportManager.toCsv(tasks)
                    }
                    requireContext().contentResolver.openOutputStream(uri)?.use { stream ->
                        stream.write(content.toByteArray(Charsets.UTF_8))
                        stream.flush()
                    } ?: throw IOException("output stream unavailable")
                    Result.success(tasks.size)
                } catch (e: kotlinx.coroutines.CancellationException) {
                    throw e
                } catch (e: Exception) {
                    Result.failure<Int>(e)
                }
            }
            result.onSuccess { count ->
                Toast.makeText(
                    requireContext(),
                    getString(R.string.settings_export_success, count),
                    Toast.LENGTH_SHORT
                ).show()
            }.onFailure {
                Toast.makeText(requireContext(), R.string.settings_export_failed, Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun importFrom(uri: Uri) {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = withContext(Dispatchers.IO) {
                try {
                    val json = requireContext().contentResolver.openInputStream(uri)?.use { stream ->
                        stream.readBytes().toString(Charsets.UTF_8)
                    } ?: throw IOException("input stream unavailable")
                    val parsed = importExportManager.parseImport(json)
                    var imported = 0
                    for (item in parsed.valid) {
                        // 复用离线创建链路：写入本地库并入 delta 队列，联网后自动同步
                        val created = taskRepository.createTask(
                            title = item.title.orEmpty().trim(),
                            description = item.description.orEmpty(),
                            status = item.status ?: "todo",
                            priority = item.priority ?: "medium",
                            dueAt = item.dueAt?.takeIf { it.isNotBlank() }
                        )
                        if (created.isSuccess) {
                            imported++
                        }
                    }
                    Result.success(imported to parsed.invalid)
                } catch (e: kotlinx.coroutines.CancellationException) {
                    throw e
                } catch (e: Exception) {
                    Result.failure<Pair<Int, Int>>(e)
                }
            }
            result.onSuccess { (imported, skipped) ->
                val message = if (skipped > 0) {
                    getString(R.string.settings_import_partial, imported, skipped)
                } else {
                    getString(R.string.settings_import_success, imported)
                }
                Toast.makeText(requireContext(), message, Toast.LENGTH_SHORT).show()
            }.onFailure {
                Toast.makeText(requireContext(), R.string.settings_import_failed, Toast.LENGTH_SHORT).show()
            }
        }
    }

    override fun onDestroyView() {
        super.onDestroyView()
        _binding = null
    }
}
