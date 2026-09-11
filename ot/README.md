# DilmeterOT

> **正式发布版本。** DilmeterOT 与 DilmeterCN、DilmeterRT 一起提供下载和在线更新。

这是 DilmeterCN 的独立自定义服务器版本。程序允许用户填写 IPv4/CIDR 网段和一个或多个服务器端口，并使用独立配置目录，不影响 DilmeterCN。

当前版本：**DilmeterOT v1.4.2**

## 直接使用

1. 确认 Windows 已安装 [Npcap](https://npcap.com/#download)，安装时建议勾选 WinPcap API 兼容模式。
2. 双击 `DilmeterOT-v1.4.2.exe`。不需要先启动洛奇。
3. 点击右上角服务器按钮，填写 IPv4/CIDR 网段与端口；设置会自动保存。
4. 使用 UU 等加速器时，勾选右上角的“加速器兼容”。
5. 程序显示“等待游戏”时可以保持开启；启动洛奇并进入服务器后，请在游戏内切换一次地图以触发捕捉。
6. 战斗产生伤害后，在报告中选择首领与角色。
7. 点击“导出日志”可把当前首领场次及所有参战角色保存为标准 UTF-8 `.json` 文件。
8. 点击“查看记录”可按日期浏览、收藏或删除 `data\logs` 中的历史记录；详情支持选择目标和队员，并可返回实时监测。
9. 点击“复制图片”可直接粘贴到聊天软件；若系统不允许访问剪贴板，程序会自动改为保存 PNG。

完整说明请查看 [使用说明.md](./使用说明.md)。

Windows 10/11 通常已经自带 Microsoft Edge WebView2 Runtime。若程序提示缺少 WebView2，请安装微软官方运行库后重试。

## 桌面版特性

- EXE 可脱离游戏独立启动，并持续等待 `Client.exe`。
- 游戏启动、退出或重连时自动切换监测状态。
- 内置桌面窗口，不打开 CMD 和外部浏览器。
- HTTP 服务仅监听 `127.0.0.1`，不会把战斗日志暴露到局域网。
- 数据资料默认固定为 CN Region / CN Lang。
- 支持填写单个 IPv4、IPv4 CIDR 网段及多个端口，设置会自动保存并在运行中生效。
- v1.2.32 保留加速器本地代理、虚拟网卡和路由模式候选连接检测，并同步当前 Buff、技能 CD、目标时间与统计修复。
- 路由模式的 Raw IPv4 数据可以直接解析，GUI 诊断日志会优先写入文件。
- 内置构建时的 CN 技能、状态与首领资料，首次打开不依赖资料站网络。
- 以单页仿游戏窗口展示战斗摘要和技能详情，不再保留高级分析标签页。
- 每个技能使用独立表格行、真实技能图标和按第一名等比例缩放的绿色伤害条。
- 技能次数不使用时间阈值合并；每条有效伤害记录计为一次。
- 人偶造成的伤害会归入操纵该人偶的玩家，并保留原技能 ID；人偶技能名称采用国服译名，报表不显示“AI”后缀。
- 推测职业覆盖国服当前 10 个阿尔卡纳，包括旋律操纵师与狂怒斗士，并按各职业完整技能编号区段识别。
- 战斗目标显示客户端观察到的累计伤害；对缺失目标出现封包的场景会保证首击只统计一次。
- 玩家统计不计普通宠物的攻击和技能伤害；人偶技能伤害仍归属其玩家主人。
- “综合”页将被动伤害拆分为特性伤害（连击）与星尘伤害（轰击、爆闪）。
- 分享图片固定为 1200 × 666，显示前 10 个伤害技能。
- 可把当前选中的首领场次保存为可读的标准 JSON；导入后自动恢复首领、角色与完整技能统计。
- 导入记录时进入独立回放状态，实时数据不会混入；点击“返回实时”即可重新载入当前监测日志。
- EXE、窗口和任务栏使用原创洛奇风编结与 DPS 上升图表图标。

## 数据位置

配置、运行日志、抓包日志、自定义音效和 WebView2 数据保存在 EXE 旁边的便携目录：

```text
<软件所在文件夹>\data\
```

其中战斗日志位于 `data\logs`，WebView2 用户数据位于 `data\webview2`。首次启动新版时，程序会把旧 `%AppData%\DilmeterOT` 中尚未存在的新目录文件复制过来；旧文件会保留，不会自动删除。

## 从源码重新构建

环境：Node.js 20+、Go 1.24.11+、Windows x64。

```text
build.bat           构建前端并复制到 Go 内置资源目录
build_backend.bat   生成无命令行窗口的 DilmeterOT-v1.4.2.exe
package_release.bat 生成 UTF-8 ZIP 与在线更新清单
npm run verify:battle-record  验证战斗记录保存与导入往返一致
```

更新内置 CN 资料时，把 Prilus 的两个文件放到相同路径后重新执行 `build.bat`：

```text
front/public/local-res/resourcedata/cn/cn_resourcedata.bin.br
front/public/local-res/resourceversion/cn/cn_resourceversion.json
```

更新内置技能图标时，在 PowerShell 中运行：

```text
.\update_skill_icons.ps1
```

脚本会从 NogiNogi 百科索引同步 CN 技能图标，并将 58009、58100、58101 三个技能替换为 Prilus 的 KR 区图标。

资料来源：[Prilus Mabi Tool](https://prilus.gitlab.io/)；报告展示方式参考 [Noginogi DPS Library](https://gear.noginogi.sbs/library.html?tab=dps)。

## 说明

本工具只能在洛奇客户端已经连接服务器并产生相应网络数据时监测实时伤害。独立启动表示程序不依赖游戏进程才能打开，不代表游戏未运行时也能产生战斗数据。
