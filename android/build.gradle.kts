plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.ksp)
    alias(libs.plugins.hilt)
}

android {
    namespace = "com.todoapp"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.todoapp"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // 服务端地址：构建时可用 -PSERVER_URL=https://your-host/api/v1/ 覆盖，默认指向模拟器宿主
        buildConfigField("String", "SERVER_URL", "\"${project.findProperty("SERVER_URL") ?: "http://10.0.2.2:8080/api/v1/"}\"")

        ndk {
            // arm64 真机 + x86_64 模拟器（CI E2E 与本地模拟器均可安装运行）
            abiFilters += listOf("arm64-v8a", "x86_64")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
            // 临时使用 debug 签名产出"内部验证包"，解决无签名包无法安装的问题。
            // 正式发布前必须替换为正式证书（经 keystore.properties / CI 环境变量注入，勿提交仓库）；
            // 内部验证包与正式包签名不同，切换时用户需卸载重装。
            signingConfig = signingConfigs.getByName("debug")
        }
        debug {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        viewBinding = true
        buildConfig = true
    }

    lint {
        // WorkManager 已升级 2.9.1 并使用 on-demand 初始化；
        // 依赖合并 manifest 中残留的 WorkManagerInitializer 与实际运行时不符，属误报
        disable += "RemoveWorkManagerInitializer"
    }

    splits {
        abi {
            isEnable = false
        }
    }

    packaging {
        resources {
            excludes += listOf(
                "META-INF/DEPENDENCIES",
                "META-INF/LICENSE",
                "META-INF/LICENSE.txt",
                "META-INF/NOTICE",
                "META-INF/NOTICE.txt"
            )
        }
    }
}

// Room schema 导出：数据库迁移演进依赖导出的 JSON 做断言校验，随仓库提交
ksp {
    arg("room.schemaLocation", "$projectDir/schemas")
}

// release 构建必须显式指定正式服务端地址，防止把模拟器占位地址打进生产包。
// 校验锚定在 release 制品任务的 doFirst：只有真正执行 assembleRelease 等任务时才检查，
// 不影响 assembleDebug、./gradlew test（含 testReleaseUnitTest）等其它任务
afterEvaluate {
    tasks.matching {
        it.name == "assembleRelease" ||
            it.name == "bundleRelease" ||
            it.name == "connectedReleaseAndroidTest"
    }.configureEach {
        doFirst {
            if (project.findProperty("SERVER_URL") == null) {
                throw GradleException(
                    "release 构建需要通过 -PSERVER_URL=https://your-host/api/v1/ 指定正式服务端地址"
                )
            }
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.appcompat)
    implementation(libs.material)
    implementation(libs.androidx.constraintlayout)

    implementation(libs.androidx.lifecycle.viewmodel.ktx)
    implementation(libs.androidx.lifecycle.livedata.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)

    implementation(libs.androidx.room.runtime)
    implementation(libs.androidx.room.ktx)
    ksp(libs.androidx.room.compiler)

    implementation(libs.androidx.work.runtime.ktx)

    implementation(libs.retrofit)
    implementation(libs.retrofit.converter.gson)
    implementation(libs.okhttp.logging)

    implementation(libs.kotlinx.coroutines.android)

    implementation(libs.androidx.security.crypto)

    implementation(libs.mlkit.barcode)
    implementation(libs.androidx.camera.core)
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.camera.view)

    implementation(libs.androidx.navigation.fragment.ktx)
    implementation(libs.androidx.navigation.ui.ktx)

    // Hilt Dependency Injection
    implementation(libs.hilt.android)
    ksp(libs.hilt.android.compiler)

    testImplementation(libs.junit)
    testImplementation(libs.robolectric)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.mockk)
    testImplementation(libs.androidx.arch.core.testing)
    testImplementation(libs.androidx.test.core.ktx)
    testImplementation(libs.androidx.work.testing)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.rules)
    androidTestImplementation(libs.espresso.core)
    androidTestImplementation(libs.kotlinx.coroutines.test)
}
