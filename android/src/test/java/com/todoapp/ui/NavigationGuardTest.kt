package com.todoapp.ui

import com.todoapp.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class NavigationGuardTest {

    @Test
    fun `配对页与登录页判定为认证屏`() {
        assertTrue(NavigationGuard.isAuthScreen(R.id.pairingFragment))
        assertTrue(NavigationGuard.isAuthScreen(R.id.loginFragment))
    }

    @Test
    fun `主界面不判定为认证屏`() {
        assertFalse(NavigationGuard.isAuthScreen(R.id.taskListFragment))
        assertFalse(NavigationGuard.isAuthScreen(R.id.settingsFragment))
    }

    @Test
    fun `未配对时强制跳转配对页`() {
        assertEquals(R.id.pairingFragment, NavigationGuard.requiredDestination(false, true))
        assertEquals(R.id.pairingFragment, NavigationGuard.requiredDestination(false, false))
    }

    @Test
    fun `已配对未登录时强制跳转登录页`() {
        assertEquals(R.id.loginFragment, NavigationGuard.requiredDestination(true, false))
    }

    @Test
    fun `已配对且已登录时无需跳转`() {
        assertEquals(null, NavigationGuard.requiredDestination(true, true))
    }
}
