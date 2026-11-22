# 真机软键盘独立验收

`npm run test:ui:device` 校验归档截图及代理证据，输出真机报告。退出码 2 表示仍有项目需要设备。核心和扩展模拟器命令独立执行。

新采证使用隔离 test-device 构建，并保留该次构建路径、构建证明、设备信息和代理日志。建立 iPhone 17 Pro 真机调试会话，使用合成测试身份。

1. 打开测试房源预约弹层或找房需求页。在远程调试 appservice Console 运行 `keyboard-console-probe.js`，执行 `await __qaDeviceKeyboard.snapshot('before')` 并截屏。
2. 在 iPhone 上触摸输入框，使用 iOS 键盘输入合成值。截图并执行 `await __qaDeviceKeyboard.snapshot('focused')`。核对正高度事件、输入框和提交按钮位置。
3. 在页面滑动，截图并执行 `await __qaDeviceKeyboard.snapshot('scroll')`。若滑动触发自动收键，记录实际行为。
4. 收键后截图，执行 `await __qaDeviceKeyboard.snapshot('dismissed')`。检查零高度事件、视口及位置恢复、输入是否保留。导出 `JSON.stringify(__qaDeviceKeyboard.entries())` 并执行 `__qaDeviceKeyboard.stop()`。
5. 按场景关联日志和截图。只有对应构建的事件及布局证据齐全时才更新验收结论。真机操作仅限交互观察。
