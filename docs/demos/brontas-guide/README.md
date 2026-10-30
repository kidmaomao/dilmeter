# 二王覆盖层动态演示

16 秒、1040×660 的循环 GIF。人物移动是确定的演示脚本，提示与箭头由生产代码 `planBrontasGuide` 计算；所有射线、时间及几何均为示例，不是真实抓包。没有新增游戏操作功能。

依次展示：进入落石区、纵线躲避、横线躲避、多条射线合并判断、只有方向时的降级、数据中断后撤回箭头。示例里的 1.6/1.25/1.0 秒不能当作真实游戏固定周期。

在仓库根目录，先按 `docs/brontas-guide-handoff.md` 完成前端构建，再执行：

```powershell
$env:DILMETER_BRONTAS_DEMO_OUT = "$PWD\docs\demos\brontas-guide\frames.json"
Push-Location cn-rt
go test ./cmd/dilmeterapi -run '^TestExportBrontasGuideDemo$' -count=1
Pop-Location
Remove-Item Env:DILMETER_BRONTAS_DEMO_OUT

Push-Location docs/demos/brontas-guide
npm install
npm run render
Pop-Location
```

生成 `docs/demos/brontas-guide/brontas-guide-demo.gif` 和 6 张检查用关键帧。生成文件及依赖均已忽略，不进入 PR。

Windows 默认使用系统微软雅黑；其他系统可通过 `DILMETER_DEMO_FONT` 指定支持中文的字体文件。颜色与布局来自 `BrontasGuideOverlay.vue`，SVG 负责演示地图与导出，不替代程序里的实际覆盖层。
