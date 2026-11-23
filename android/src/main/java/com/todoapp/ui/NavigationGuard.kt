package com.todoapp.ui

import com.todoapp.R

/**
 * 导航守卫：登录/配对状态决定强制跳转目标与认证屏判定，
 * 供 MainActivity 的导航分发与底部栏可见性逻辑复用。
 */
object NavigationGuard {

    fun isAuthScreen(destinationId: Int): Boolean =
        destinationId == R.id.pairingFragment || destinationId == R.id.loginFragment

    fun requiredDestination(isUserPaired: Boolean, isUserLoggedIn: Boolean): Int? =
        when {
            !isUserPaired -> R.id.pairingFragment
            !isUserLoggedIn -> R.id.loginFragment
            else -> null
        }
}
