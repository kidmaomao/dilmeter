# Dilmeter

Dilmeter 是面向《洛奇》的 Windows 桌面伤害统计与战斗提醒工具。本仓库同时维护三个版本：

| 版本 | 源码目录 | 用途 |
| --- | --- | --- |
| DilmeterCN | `cn-rt` | 国服常规网络环境 |
| DilmeterRT | `cn-rt` | 国服路由/加速器兼容模式 |
| DilmeterOT | `ot` | 自定义服务器地址及其他服务器环境 |

CN 与 RT 固定使用随程序提供的 CN 名称和图标数据，可离线启动。OT 默认同样携带 CN 数据，并保留外置资源包覆盖能力。

当前版本为 **1.4.2**，更新文案见 [1.4.2 更新公告](docs/release-notes/v1.4.2.md)。CN、RT、OT 本地包输出到 `artifacts/release-v1.4.2`，公开下载见 [软件下载页](https://gear.noginogi.sbs/downloads/)。

## 仓库结构

```text
cn-rt/     DilmeterCN 与 DilmeterRT 共用源码、前端和离线资源
ot/        DilmeterOT 源码、前端和外置资源包说明
.github/   持续集成与 GitHub Release 发布流程
```

历史 EXE、ZIP、运行日志、浏览器缓存、测试截图和旧静态构建不会提交到仓库。GitHub Release 只保存最终发布包与更新清单。

## 开发环境

- Windows 10/11 x64
- Node.js 24
- Go 1.24.11
- `go-winres` 0.3.3
- 运行 CN/OT 时需要安装 Npcap；RT 发布包会附带 WinDivert 运行文件

安装构建工具：

```powershell
go install github.com/tc-hib/go-winres@v0.3.3
```

## 构建

构建 CN 与 RT：

```powershell
Set-Location cn-rt
cmd /c build.bat
cmd /c build_backend.bat
cmd /c build_rt.bat
```

构建 OT：

```powershell
Set-Location ot
cmd /c build.bat
cmd /c build_backend.bat
```

`build.bat` 会生成页面资源，并同步到 Go 的嵌入目录。首次构建前请分别在两个 `front` 目录运行 `npm ci`。

## 发布与更新

GitHub Actions 的 `Release Dilmeter` 流程会构建三个版本并创建或更新 GitHub Release。发布资源名保持固定：

- `DilmeterCN.zip`
- `DilmeterRT.zip`
- `DilmeterOT.zip`
- `DilmeterCN.json`、`DilmeterRT.json`、`DilmeterOT.json`

新版本客户端优先从 GitHub 最新 Release 读取更新清单并下载经过 SHA-256 校验的 ZIP；GitHub 不可用或仓库保持私有时，自动使用 NogiNogi 下载站的更新清单。NogiNogi 下载页也会自动识别同一 Release；GitHub 暂不可用时继续使用站内发布包。同步站内清单时，下载 URL 应指向站内 ZIP，文件大小和 SHA-256 必须与发布包一致。

## 资料与许可

各子目录的 README 和使用说明包含版本差异、抓包要求和资源更新方式。第三方组件的许可文件保留在对应源码目录中。本仓库未声明的游戏名称、图标和数据版权归其各自权利人所有。
