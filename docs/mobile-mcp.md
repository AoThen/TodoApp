# mobile-mcp 部署与使用

mobile-mcp 是面向移动端的 MCP（Model Context Protocol）服务，可让 AI 编码助手直接操控 Android/iOS 设备：安装应用、点击/输入、截图、读取日志等。本项目用它对 Android 端做真机验收与问题复现。

## 部署位置

mobile-mcp 部署在**本地开发机**上，通过 adb 连接真机或模拟器；GitHub Actions 的 CI 流程不使用它。

## 前置条件

1. 本地安装 Node.js 20+。
2. Android 平台工具（adb）在 PATH 中。
3. 一台已开启 USB 调试或无线调试的真机，或本地 AVD 模拟器。
4. `adb devices` 能列出目标设备。

## 启动 mobile-mcp

```bash
npx @mobile-next/mobile-mcp@latest
```

在 opencode / Claude 等 MCP 客户端中注册（stdio 方式）：

```json
{
  "mcpServers": {
    "mobile-mcp": {
      "command": "npx",
      "args": ["@mobile-next/mobile-mcp@latest"]
    }
  }
}
```

## 连接真机

```bash
# USB 调试：插上数据线后确认设备可见
adb devices

# 无线调试（Android 11+，与手机同网段）：
adb pair <phone-ip>:<pair-port>      # 输入手机上显示的配对码
adb connect <phone-ip>:<connect-port>
```

## 与 TodoApp 联调的典型流程

1. 构建并安装 Debug 包：

   ```bash
   cd android
   ./gradlew assembleDebug
   adb install -r build/outputs/apk/debug/android-debug.apk
   ```

2. 本地启动后端（真机需用电脑局域网 IP）：

   ```bash
   JWT_SECRET=... DEVICE_KEY_KEK=... INITIAL_ADMIN_EMAIL=... INITIAL_ADMIN_PASSWORD=... \
   go run . # 监听 8080
   ```

3. 应用内配对：扫描 Web 端“设备配对”二维码完成绑定。
4. 用 mobile-mcp 的工具驱动手机：截图、点击任务项、输入新任务、验证同步结果。
5. 抓取日志定位问题：`adb logcat --pid=$(adb shell pidof -s com.todoapp)`。

## 常用 mobile-mcp 能力

- 列出/选择设备，安装、启动、终止应用
- 屏幕截图与层级结构获取，辅助 AI 理解界面
- 点击坐标、滑动、输入文本、按键
- 获取应用日志，辅助崩溃与同步问题排查

## 注意事项

- 真机测试时 `SERVER_URL` 必须是电脑局域网 IP，不能用 `10.0.2.2`。
- 设备配对与加密依赖每设备独立密钥：服务端使用 `DEVICE_KEY_KEK` 在库内封装保存，客户端在配对时经 ECDH 领取各自的密钥。
- mobile-mcp 只应连接你自己的测试设备，不要对他人设备执行操作。
