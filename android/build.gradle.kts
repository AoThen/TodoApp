import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.ksp)
    alias(libs.plugins.hilt)
}

// 正式签名：CI 经环境变量注入（KEYSTORE_FILE/KEYSTORE_PASSWORD/KEY_ALIAS/KEY_PASSWORD），
// 本地可放 android/keystore.properties（已被 .gitignore 忽略，勿提交仓库）
val keystoreProps = Properties().apply {
    rootProject.file("keystore.properties").takeIf { it.exists() }
        ?.inputStream()?.use { load(it) }
}

android {
    namespace = "com.todoapp"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.todoapp"
        minSdk = 26
        targetSdk = 34
        // CI release 构建经 -PVERSION_CODE/-PVERSION_NAME 注入（run_number 单调递增），本地默认 1/1.0
        versionCode = (project.findProperty("VERSION_CODE") ?: "1").toString().toInt()
        versionName = project.findProperty("VERSION_NAME")?.toString() ?: "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // 服务端地址：构建时可用 -PSERVER_URL=https://your-host/api/v1/ 覆盖，默认指向模拟器宿主
        buildConfigField("String", "SERVER_URL", "\"${project.findProperty("SERVER_URL") ?: "http://10.0.2.2:8080/api/v1/"}\"")

        ndk {
            // arm64 真机 + x86_64 模拟器（CI E2E 与本地模拟器均可安装运行）
            abiFilters += listOf("arm64-v8a", "x86_64")
        }
    }

    signingConfigs {
        create("release") {
            val storePath = System.getenv("KEYSTORE_FILE") ?: keystoreProps.getProperty("storeFile")
            if (storePath != null) {
                storeFile = file(storePath)
                storePassword = System.getenv("KEYSTORE_PASSWORD") ?: keystoreProps.getProperty("storePassword")
                keyAlias = System.getenv("KEY_ALIAS") ?: keystoreProps.getProperty("keyAlias")
                keyPassword = System.getenv("KEY_PASSWORD") ?: keystoreProps.getProperty("keyPassword")
            }
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
            // 四项签名信息齐全才用正式证书；否则回退 debug 签名产出"内部验证包"。
            // 内部验证包与正式包签名不同，切换时用户需卸载重装。
            val releaseSigning = signingConfigs.getByName("release")
            val hasReleaseSigning = releaseSigning.storeFile != null &&
                releaseSigning.storePassword != null &&
                releaseSigning.keyAlias != null &&
                releaseSigning.keyPassword != null
            if (hasReleaseSigning) {
                signingConfig = releaseSigning
            } else {
                logger.warn("release 未提供完整正式签名，回退 debug 签名（仅限本地内部验证，禁止分发）")
                signingConfig = signingConfigs.getByName("debug")
            }
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
            val serverUrl = project.findProperty("SERVER_URL")?.toString()
            if (serverUrl.isNullOrBlank()) {
                throw GradleException(
                    "release 构建需要通过 -PSERVER_URL=https://your-host/api/v1/ 指定正式服务端地址"
                )
            }
            val signing = signingConfigs.getByName("release")
            val hasReleaseSigning = signing.storeFile != null &&
                signing.storePassword != null &&
                signing.keyAlias != null &&
                signing.keyPassword != null
            if (project.findProperty("requireReleaseSigning") == "true" && !hasReleaseSigning) {
                throw GradleException(
                    "release 构建要求正式签名：请配置 GitHub Secrets " +
                        "(KEYSTORE_BASE64/KEYSTORE_PASSWORD/KEY_ALIAS/KEY_PASSWORD) 或本地 keystore.properties"
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
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.androidx.arch.core.testing)
    testImplementation(libs.androidx.test.core.ktx)
    testImplementation(libs.androidx.work.testing)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.rules)
    androidTestImplementation(libs.espresso.core)
    androidTestImplementation(libs.kotlinx.coroutines.test)
}
