#!/usr/bin/env bash
# release 包冷启动冒烟：验证 R8 混淆 + 正式签名包可正常启动
# 由 CI emulator-runner 以单行命令调用（emulator-runner 的 script 逐行拆分为独立 sh -c，
# 多行脚本无法共享状态，必须整体封装为仓库脚本）
set -u

PKG="com.todoapp"
ACTIVITY="$PKG/.ui.MainActivity"
APK="android/build/outputs/apk/release/app-release.apk"
FAILED=0

if [ ! -f "$APK" ]; then
  echo "FAIL: 未找到 release APK: $APK"
  exit 1
fi

adb install -r "$APK" || exit 1

smoke() {
  local label="$1"
  echo "===== release 启动冒烟: $label ====="
  adb logcat -c
  adb shell am start -W -n "$ACTIVITY"
  sleep 10
  PID=$(adb shell pidof "$PKG" | tr -d ' \r' || true)
  adb logcat -d -b crash > "crash_release_${label}.log" || true
  if [ -z "$PID" ]; then
    echo "FAIL $label: 启动后进程已死亡，崩溃栈如下："
    tail -80 "crash_release_${label}.log" || true
    FAILED=1
  else
    echo "PASS $label: 进程存活 pid=$PID"
  fi
  adb shell am force-stop "$PKG"
}

# 两轮冷启动，提高时序/竞态类崩溃的捕获率
smoke round1
smoke round2

if [ "$FAILED" -eq 1 ]; then
  echo "release 冒烟失败"
  exit 1
fi
echo "release 冒烟通过"
