package com.todoapp.di

import android.content.Context
import com.todoapp.data.remote.ApiService
import com.todoapp.data.remote.RetrofitClient
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {

    @Provides
    @Singleton
    fun provideApiService(@ApplicationContext context: Context): ApiService {
        // 统一网络栈：登录、业务、同步、配对共用同一套 cookie、鉴权（含 401 自动续期）、
        // 加密拦截与 baseUrl 切换逻辑，避免双栈行为不一致、重新配对后地址漂移
        return RetrofitClient.getApiService(context)
    }
}
