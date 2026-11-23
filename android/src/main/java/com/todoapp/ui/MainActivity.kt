package com.todoapp.ui

import android.os.Bundle
import android.view.View
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.isVisible
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.navigation.NavController
import androidx.navigation.NavOptions
import androidx.navigation.fragment.NavHostFragment
import androidx.navigation.ui.setupWithNavController
import com.google.android.material.bottomnavigation.BottomNavigationView
import com.todoapp.R
import com.todoapp.TodoApp
import com.todoapp.data.notify.NotificationSocketController
import com.todoapp.data.remote.RetrofitClient
import com.todoapp.data.remote.SessionEvents
import com.todoapp.utils.JwtUtils
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

@AndroidEntryPoint
class MainActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        setupNavigation()
        observeSessionExpiry()
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

        // 统一导航守卫：覆盖冷启动首帧、底栏点击、进程恢复等所有入口，
        // 防止未配对/未登录用户经底栏或深链绕过配对/登录页进入主界面
        navController.addOnDestinationChangedListener { controller, destination, _ ->
            val app = application as TodoApp
            val isAuthScreen = NavigationGuard.isAuthScreen(destination.id)

            // 配对/登录页隐藏底部导航，避免未登录用户误入任务列表/设置页（设置页含退出登录入口）
            bottomNav?.isVisible = !isAuthScreen

            if (isAuthScreen) return@addOnDestinationChangedListener

            val requiredDestination = NavigationGuard.requiredDestination(
                app.isUserPaired(),
                app.isUserLoggedIn()
            )
            if (requiredDestination != null && controller.currentDestination?.id != requiredDestination) {
                // post 到下一帧执行，避免在导航分发过程中再次 navigate 引发状态异常
                bindingRoot().post {
                    if (isDestroyed || isFinishing) return@post
                    val navHost = supportFragmentManager
                        .findFragmentById(R.id.nav_host_fragment) as? NavHostFragment
                        ?: return@post
                    val nc = navHost.navController
                    if (nc.currentDestination?.id != requiredDestination) {
                        navigateClearingBackStack(nc, requiredDestination)
                    }
                }
            } else if (requiredDestination == null && destination.id == R.id.taskListFragment) {
                // 已进入主界面：access token 过期则后台静默续期（不阻塞首屏，本地数据照常展示）
                refreshTokenInBackgroundIfExpired()
            }
        }

        // 注意：主题为 NoActionBar，不能调用 setupActionBarWithNavController——
        // 它内部会立即回调 setTitle 并要求 setSupportActionBar 已设置，
        // 否则抛 IllegalStateException 导致冷启动闪退（CI 冒烟已复现）
    }

    private fun bindingRoot(): View = findViewById(android.R.id.content)

    private fun navigateClearingBackStack(navController: NavController, destinationId: Int) {
        // 清空整个回退栈，避免按返回键退回已失效的登录/主界面
        // 注意 Navigation 2.7.6 无 navigate(Int, NavOptions) / KDSL 重载，
        // 需走经典三参 navigate(resId, args, navOptions)
        val options = NavOptions.Builder()
            .setPopUpTo(0, true)
            .build()
        navController.navigate(destinationId, null, options)
    }

    /**
     * access token 已过期时后台静默刷新：
     * - 成功：无感续期
     * - 服务端明确拒绝（refresh token 失效）：清会话并跳登录页
     * - 网络瞬态失败：保留会话，等待下次请求重试
     */
    private fun refreshTokenInBackgroundIfExpired() {
        val token = RetrofitClient.getAccessToken(this)
        if (JwtUtils.validateAccessToken(token) !is JwtUtils.TokenValidation.Expired) return

        lifecycleScope.launch(Dispatchers.IO) {
            val outcome = RetrofitClient.refreshAccessToken(applicationContext)
            if (outcome == RetrofitClient.RefreshOutcome.InvalidSession) {
                RetrofitClient.clearToken(applicationContext)
                SessionEvents.notifyExpired()
            }
        }
    }

    private fun observeSessionExpiry() {
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                SessionEvents.expired.collect {
                    val navHostFragment = supportFragmentManager
                        .findFragmentById(R.id.nav_host_fragment) as? NavHostFragment
                        ?: return@collect
                    val navController = navHostFragment.navController
                    if (navController.currentDestination?.id == R.id.loginFragment) {
                        return@collect
                    }
                    Toast.makeText(
                        this@MainActivity,
                        R.string.session_expired_relogin,
                        Toast.LENGTH_LONG
                    ).show()
                    navigateClearingBackStack(navController, R.id.loginFragment)
                }
            }
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
