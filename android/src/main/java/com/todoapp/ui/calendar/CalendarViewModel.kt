package com.todoapp.ui.calendar

import android.content.Context
import android.widget.Toast
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.todoapp.R
import com.todoapp.data.local.Task
import com.todoapp.data.repository.TaskRepository
import com.todoapp.utils.DateTimeUtils
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.LocalDate
import java.time.YearMonth
import javax.inject.Inject

/** 月历网格中的单个日期格子 */
data class CalendarDay(
    val date: LocalDate,
    val inMonth: Boolean,
    val isToday: Boolean,
    val isSelected: Boolean,
    val taskCount: Int
)

data class CalendarUiState(
    val yearMonth: YearMonth = YearMonth.now(),
    val selectedDate: LocalDate = LocalDate.now(),
    val days: List<CalendarDay> = emptyList(),
    val dayTasks: List<Task> = emptyList()
)

@HiltViewModel
class CalendarViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val taskRepository: TaskRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(CalendarUiState())
    val uiState: StateFlow<CalendarUiState> = _uiState.asStateFlow()

    /** 当前选中月份的全部带截止日期任务（按 dueAt 升序） */
    private var monthTasks: List<Task> = emptyList()

    init {
        refreshMonthTasks()
    }

    fun previousMonth() = shiftMonth(-1)

    fun nextMonth() = shiftMonth(1)

    fun selectDate(date: LocalDate) {
        _uiState.update { it.copy(selectedDate = date) }
        rebuildDays()
        loadSelectedDayTasks()
    }

    fun toggleTaskCompletion(task: Task) {
        viewModelScope.launch {
            taskRepository.toggleTaskCompletion(task)
                .onSuccess { refreshMonthTasks() }
                .onError { e ->
                    Toast.makeText(
                        context,
                        context.getString(R.string.task_update_failed, e.message),
                        Toast.LENGTH_SHORT
                    ).show()
                }
        }
    }

    fun deleteTask(task: Task) {
        viewModelScope.launch {
            taskRepository.deleteTask(task)
                .onSuccess { refreshMonthTasks() }
                .onError { e ->
                    Toast.makeText(
                        context,
                        context.getString(R.string.task_delete_failed, e.message),
                        Toast.LENGTH_SHORT
                    ).show()
                }
        }
    }

    private fun shiftMonth(delta: Long) {
        _uiState.update { state ->
            val newMonth = state.yearMonth.plusMonths(delta)
            val newSelected = if (YearMonth.from(state.selectedDate) == newMonth) {
                state.selectedDate
            } else {
                newMonth.atDay(1)
            }
            state.copy(yearMonth = newMonth, selectedDate = newSelected)
        }
        refreshMonthTasks()
    }

    /** 按当前月份从本地库重新加载带截止日期的任务 */
    private fun refreshMonthTasks() {
        val ym = _uiState.value.yearMonth
        viewModelScope.launch {
            val start = DateTimeUtils.formatDateTime(ym.year, ym.monthValue - 1, 1, 0, 0)
            val end = DateTimeUtils.formatDateTime(ym.year, ym.monthValue - 1, ym.lengthOfMonth(), 23, 59)
            taskRepository.getTasksDueBetween(start, end).onSuccess { tasks ->
                monthTasks = tasks
                rebuildDays()
                loadSelectedDayTasks()
            }
        }
    }

    /** 重建 6 周（42 格）月历网格，附带选中/今日/任务数标记 */
    private fun rebuildDays() {
        val state = _uiState.value
        val ym = state.yearMonth
        val today = LocalDate.now()

        val countByDate = monthTasks
            .mapNotNull { t -> t.dueAt?.let { runCatching { LocalDate.parse(it.substring(0, 10)) }.getOrNull() } }
            .groupingBy { it }
            .eachCount()

        val cells = mutableListOf<CalendarDay>()

        // 上月补位（周一起始）
        val offset = ym.atDay(1).dayOfWeek.value - 1
        val prevMonthEnd = ym.minusMonths(1).atEndOfMonth()
        for (i in offset downTo 1) {
            val date = prevMonthEnd.minusDays((i - 1).toLong())
            cells.add(CalendarDay(date, false, date == today, date == state.selectedDate, 0))
        }

        for (day in 1..ym.lengthOfMonth()) {
            val date = ym.atDay(day)
            cells.add(CalendarDay(date, true, date == today, date == state.selectedDate, countByDate[date] ?: 0))
        }

        // 下月补齐到整周
        val tail = (7 - cells.size % 7) % 7
        val nextMonthStart = ym.plusMonths(1).atDay(1)
        for (i in 0 until tail) {
            val date = nextMonthStart.plusDays(i.toLong())
            cells.add(CalendarDay(date, false, date == today, date == state.selectedDate, 0))
        }

        _uiState.update { it.copy(days = cells) }
    }

    private fun loadSelectedDayTasks() {
        val prefix = _uiState.value.selectedDate.toString()
        _uiState.update { state ->
            state.copy(dayTasks = monthTasks.filter { it.dueAt?.startsWith(prefix) == true })
        }
    }
}
