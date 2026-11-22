package com.todoapp.ui.settings

import android.net.Uri
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
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
import com.todoapp.data.repository.AuthRepository
import com.todoapp.data.repository.TaskRepository
import com.todoapp.databinding.FragmentSettingsBinding
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

        // 应用版本号
        binding.tvVersion.text = BuildConfig.VERSION_NAME

        // 重新配对设备
        binding.btnRePair.setOnClickListener {
            findNavController().navigate(R.id.action_settingsFragment_to_pairingFragment)
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
