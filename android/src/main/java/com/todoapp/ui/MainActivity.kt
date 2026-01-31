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
            .findFragmentById(R.id.nav_host_fragment) as NavHostFragment
        val navController = navHostFragment.navController

        // Setup bottom navigation
        val bottomNav = findViewById<BottomNavigationView>(R.id.bottom_navigation)
        bottomNav?.setupWithNavController(navController)

        // Setup toolbar with navigation
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
            .findFragmentById(R.id.nav_host_fragment) as NavHostFragment
        val navController = navHostFragment.navController

        when {
            !app.isUserPaired() -> {
                // Navigate to pairing if not paired
                navController.navigate(R.id.pairingFragment)
            }
            !app.isUserLoggedIn() -> {
                // Navigate to login if paired but not logged in
                navController.navigate(R.id.loginFragment)
            }
            else -> {
                // User is paired and logged in, go to main app
                navController.navigate(R.id.taskListFragment)
            }
        }
    }

    override fun onSupportNavigateUp(): Boolean {
        val navHostFragment = supportFragmentManager
            .findFragmentById(R.id.nav_host_fragment) as NavHostFragment
        return navHostFragment.navController.navigateUp() || super.onSupportNavigateUp()
    }
}