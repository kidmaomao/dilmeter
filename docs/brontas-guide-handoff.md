# 换电脑继续开发

工作分支：`feat/brontas-native-guide`。PR 保持草稿，覆盖层已接入，射线识别尚未接通。不要直接合并作为实战功能发布。

```powershell
git clone https://github.com/kidmaomao/dilmeter.git
cd dilmeter
git fetch origin
git switch --track origin/feat/brontas-native-guide
```

已有仓库则在保存本机未提交修改后，执行最后两条命令；本地已有同名分支时用 `git switch feat/brontas-native-guide`。

## 先构建和检查

CN 与 RT 共用 `cn-rt`，OT 位于 `ot`。本次验证使用 Node.js 24.15 与 Go 1.26.1。接续时可使用同版本；前端校验脚本直接导入 TypeScript，因此不要只按旧 `volta` 字段安装 Node.js 20。Go 依赖以项目 `go.mod` 为准。

```powershell
cd cn-rt/front
npm ci
npx vue-tsc --noEmit
npm run build -- --outDir ../cmd/dilmeterapi/static_v130_release
npm run verify:brontas-guide
npm run verify:monitor-extensions
npm run verify:boss-mechanic-alert
npm run verify:reminder-profiles
cd ..
go test ./...
go build -ldflags '-H=windowsgui -X main.BuildVariant=brontas-guide-dev' -o DilmeterCN-Brontas.exe ./cmd/dilmeterapi
```

在 OT 做相同修改/验证并保留繁体界面。测试过程中曾有一项更新下载测试因执行环境禁止本机网络连接而失败；家用电脑应先运行完整测试，不要默认跳过。其余后端检查、两版类型检查、核心提醒回归及 CN/RT 职业 KPI/战斗记录检查已经通过。

## 下一步入口

- `bossBrontasGuide.go`：观察快照、路径判断、倒计时、超时撤回、本机 `/api/brontas_guide` 输入。
- `bossBrontasRocks.go`：实际矿石生命周期与 50% 阶段关联。
- `BrontasGuideOverlay.vue`：Dilmeter 透明 Windows 窗口里的 UI。
- `bossBrontasGuide_test.go`：核心回归。
- `bossBrontasGuideDemo_test.go` 与 `docs/demos/brontas-guide`：可重建的动态图。
- `docs/brontas-guide.md`：完整输入格式、坐标定义、降级与边界。

优先补齐：

1. 定位已有的**二王**射线识别代码/分支/抓包。主分支 `publishBossLaserPacket` 处理的是三王技能 `52401`，不能直接当二王输入。
2. 先接入射线方向和致命绝对时刻，验证真实倒计时；结束时刻未知时保留持续危险，不能猜测结束时间。
3. 确认人物坐标、落石区半径、射线宽度、移动速度和对侧射线的同一坐标系，再启用具体落点。
4. 实测延迟、不同入场方向、掉线/死亡/转场、镜头方向、DPI、鼠标穿透和全屏/无边框显示。区外路径和世界地面投影当前没有实现。

当前反应预算 300 ms、时间余量 150 ms、空间余量 0.03 均是工程初值，需游戏内校准。不要根据演示动画反推机制规则。

正式发布前仍须遵守根目录 `AGENTS.md` 和 `docs/releasing.md`。本 PR 不包含安装包、个人日志、抓包或配置，也不会自动替代正式版。
