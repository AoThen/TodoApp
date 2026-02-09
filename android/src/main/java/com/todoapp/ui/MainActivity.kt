package com.todoapp.ui

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.navigation.fragment.NavHostFragment
import androidx.navigation.ui.AppBarConfiguration
import androidx.navigation.ui.setupActionBarWithNavController
import androidx.navigation.ui.setupWithNavController
import com.google.android.material.bottomnavigation.BottomNavigationView
import com.todoapp.R
import com.todoapp.TodoApp

class MainActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        setupNavigation()
        
        // Check initial navigation state
        checkInitialNavigation()
    }

    private fun setupNavigation() {
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment)
        if (navHostFragment !is NavHostFragment) return
        val navController = navHostFragment.navController

        val bottomNav = findViewById<BottomNavigationView>(R.id.bottom_navigation)
        bottomNav?.setupWithNavController(navController)

        val appBarConfiguration = AppBarConfiguration(
            setOf(
                R.id.taskListFragment,
                R.id.calendarFragment,
                R.id.settingsFragment
            )
        )
        setupActionBarWithNavController(navController, appBarConfiguration)
    }

    private fun checkInitialNavigation() {
        val app = application as TodoApp
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment)
        if (navHostFragment !is NavHostFragment) return
        val navController = navHostFragment.navController

        when {
            !app.isUserPaired() -> {
                navController.navigate(R.id.pairingFragment)
            }
            !app.isUserLoggedIn() -> {
                navController.navigate(R.id.loginFragment)
            }
            else -> {
                navController.navigate(R.id.taskListFragment)
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