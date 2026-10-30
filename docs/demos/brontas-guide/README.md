# 二王覆盖层动态演示

16 秒、1040×660 的循环 GIF。人物移动是确定的演示脚本，提示与箭头由生产代码 `planBrontasGuide` 计算；所有射线、时间及几何均为示例，不是真实抓包。没有新增游戏操作功能。

依次展示：进入落石区、纵线躲避、横线躲避、多条射线合并判断、只有方向时的降级、数据中断后撤回箭头。示例里的 1.6/1.25/1.0 秒不能当作真实游戏固定周期。

射线宽度根据用户提供的圆形九宫格草图修正：纵带约为圆直径的 40%，横带约为 51%；交叉示例的横带中心向下偏移 0.12 个半径，形成两个较小的下方角落。`halfWidth / 圆半径` 等于 `全宽 / 圆直径`。这些比例只用于演示与回归测试，不作为实战识别的默认值。

图中小角看得见空地，并不代表容得下人物：算法仍检查人物半径、空间余量和整段移动路径。本例下方两角无法容纳所设人物，指引选择上方区域。

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

## 连续加速版

新增完整倒计时与连续出线的 20 秒版本，详见 [视频采样、未校准参数及重建步骤](../../brontas-cadence-analysis.md)。原 16 秒功能样例仍可按上面的命令生成。
