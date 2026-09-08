# 专用浏览器

`interaction.go` 提供有界的鼠标、文本、按键及二维码截图操作。当前 rod 的部分便捷方法经由长期页面上下文等待动画帧或发送输入，可能忽略请求取消；因此这些路径改为带请求上下文的 CDP 指令与计时稳定性检查。控件必须可见、可交互，点击不会穿透遮挡或自动重试。

生产服务使用带所有权标记的 `data/browser-profile/` 持久化配置，不读取日常浏览器；正常关闭保留登录。仅测试可用临时无痕上下文。代理、User-Agent、可选 Stealth 实际应用到浏览器/新页面。请求结束不取消长期标签页的事件监听。重置会话清除专用资料及页面，未启动浏览器时也可清除；资料无标记、路径不符或仍被锁定时拒绝删除。

默认 `background=true, headless=false`。通过 CDP `Target.createTarget` 的 `newWindow + background + windowState=minimized` 创建专用页面，并在导航前检查浏览器确实采用最小化状态；字段不受支持时失败，不先显示再隐藏。Rod 当前生成类型尚无 `windowState`，因此仅为该调用补充标准协议字段，不修改依赖源码。协议定义见 [Chrome DevTools Target.createTarget](https://chromedevtools.github.io/devtools-protocol/tot/Target/#method-createTarget)。

普通鼠标和输入操作不调用 `Page.bringToFront`。后台页面用标准 [Emulation.setFocusEmulationEnabled](https://chromedevtools.github.io/devtools-protocol/tot/Emulation/#method-setFocusEmulationEnabled) 保持 DOM 输入和页面更新活跃，不激活操作系统窗口；仅最小化而不保留页面焦点会导致搜索输入或互动停滞。平台兼容层识别到登录/验证后，`ShowManualVerification` 关闭焦点模拟，只恢复已持有且位于抖音官网的页面；浏览器上下文保存待接管目标，不受单次请求结束影响。服务入口检查待验证页面，未完成时暂停其他操作；确认完成才调用 `ResumeBackground`，保留文档、编辑器、动作 ID 与回执，不刷新或重发。用户关掉验证窗口时，仅在浏览器确认目标不存在后清除该等待状态。

这不是运行时切换 headless/headful。显式 `headless=true` 不会自动重启为有头实例，也不能显示手动接管窗口；该模式下仍报告阻断。`background=false` 可用于持续可见的本地调试。后台模式不隐藏操作记录，也不绕过平台验证。
