package com.todoapp.ui

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.navigation.NavController
import androidx.navigation.NavOptions
import androidx.navigation.fragment.NavHostFragment
import androidx.navigation.ui.setupWithNavController
import com.google.android.material.bottomnavigation.BottomNavigationView
import com.todoapp.R
import com.todoapp.TodoApp
import com.todoapp.data.notify.NotificationSocketController
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        setupNavigation()

        // 仅在首次创建时做初始导航；旋转屏/进程恢复时 savedInstanceState 非空，
        // Navigation 已自行恢复目的地，强制跳转会把用户从日历/设置页弹回任务列表
        if (savedInstanceState == null) {
            checkInitialNavigation()
        }
    }

    override fun onStart() {
        super.onStart()
        // 前台时建立通知实时通道（未登录/未配对时内部会安全跳过）
        NotificationSocketController.start(this)
    }

    override fun onStop() {
        super.onStop()
        NotificationSocketController.stop()
    }

    private fun setupNavigation() {
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment)
        if (navHostFragment !is NavHostFragment) return
        val navController = navHostFragment.navController

        val bottomNav = findViewById<BottomNavigationView>(R.id.bottom_navigation)
        bottomNav?.setupWithNavController(navController)

        // 注意：主题为 NoActionBar，不能调用 setupActionBarWithNavController——
        // 它内部会立即回调 setTitle 并要求 setSupportActionBar 已设置，
        // 否则抛 IllegalStateException 导致冷启动闪退（CI 冒烟已复现）
    }

    private fun checkInitialNavigation() {
        val app = application as TodoApp
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment)
        if (navHostFragment !is NavHostFragment) return
        val navController = navHostFragment.navController

        when {
            !app.isUserPaired() -> {
                navigateIfNotCurrent(navController, R.id.pairingFragment)
            }
            !app.isUserLoggedIn() -> {
                navigateIfNotCurrent(navController, R.id.loginFragment)
            }
            else -> {
                navigateIfNotCurrent(navController, R.id.taskListFragment)
            }
        }
    }

    private fun navigateIfNotCurrent(navController: NavController, destinationId: Int) {
        if (navController.currentDestination?.id != destinationId) {
            // 清空到 startDestination（含）的旧栈，避免已登录用户按返回键退回配对/登录页。
            // 注意 Navigation 2.7.6 无 navigate(Int, NavOptions) / KDSL 重载，
            // 需走经典三参 navigate(resId, args, navOptions)
            val options = NavOptions.Builder()
                .setPopUpTo(R.id.pairingFragment, true)
                .build()
            navController.navigate(destinationId, null, options)
        }
    }

    override fun onSupportNavigateUp(): Boolean {
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment)
        if (navHostFragment is NavHostFragment) {
            return navHostFragment.navController.navigateUp() || super.onSupportNavigateUp()
        }
        return super.onSupportNavigateUp()
    }
}