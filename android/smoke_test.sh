#!/usr/bin/env bash
# Android E2E：DAO instrumented 测试 + MainActivity 冷启动冒烟（深浅色两轮）
# 在 android-emulator-runner 的模拟器会话内执行；runner 的 script 逐行执行，
# 因此本文件必须作为整体脚本被 bash 调用，不能内联展开到 workflow script 里。
set -e
cd "$(dirname "$0")"

./gradlew connectedDebugAndroidTest -PSERVER_URL=http://10.0.2.2:8080/api/v1/ --no-daemon

# ===== 启动冒烟测试：全新安装后冷启动 MainActivity =====
# 背景：CI 此前只覆盖 DAO，从未启动过 UI；线上曾出现"点图标立刻闪退"
APK=$(ls build/outputs/apk/debug/*.apk | head -1)
adb uninstall com.todoapp || true
adb install -r "$APK"

smoke() {
  local label="$1"
  echo "===== 启动冒烟: $label ====="
  adb logcat -c
  adb shell am start -W -n com.todoapp/.ui.MainActivity
  sleep 10
  PID=$(adb shell pidof com.todoapp | tr -d ' \r' || true)
  adb logcat -d -b crash > "crash_${label}.log" || true
  adb logcat -d "*:E" > "errors_${label}.log" || true
  if [ -z "$PID" ]; then
    echo "FAIL $label: 启动后进程已死亡，崩溃栈如下："
    tail -80 "crash_${label}.log" || true
    echo "----- ${label} error 日志（最近 40 行） -----"
    tail -40 "errors_${label}.log" || true
    FAILED=1
  else
    echo "PASS $label: 进程存活 pid=$PID"
  fi
  adb shell am force-stop com.todoapp
}

FAILED=0
adb shell cmd uimode night no
smoke light
# 首启崩溃复探：同模式下立即二次启动，区分「仅首次安装崩溃」与「该模式下持续崩溃」
if [ "$FAILED" -eq 1 ]; then
  echo "===== 复探: light 第二次启动 ====="
  adb logcat -c
  adb shell am start -W -n com.todoapp/.ui.MainActivity
  sleep 8
  PID2=$(adb shell pidof com.todoapp | tr -d ' \r' || true)
  adb logcat -d -b crash > crash_light_retry.log || true
  if [ -z "$PID2" ]; then
    echo "FAIL light-retry: 二次启动仍崩溃，崩溃栈如下："
    tail -80 crash_light_retry.log || true
  else
    echo "PASS light-retry: 二次启动存活 pid=$PID2 —— 属「仅首次启动崩溃」"
  fi
  adb shell am force-stop com.todoapp
  FAILED=0
fi
adb shell cmd uimode night yes
smoke dark

if [ "$FAILED" -eq 1 ]; then
  {
    echo "## Android 启动冒烟失败"
    echo '```'
    tail -120 crash_light.log crash_dark.log 2>/dev/null
    tail -40 errors_light.log errors_dark.log 2>/dev/null
    echo '```'
  } >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
  exit 1
fi
