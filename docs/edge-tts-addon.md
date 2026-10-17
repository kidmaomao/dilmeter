# 晓晓语音附加包

为只需要制作自定义语音的玩家提供独立 ZIP。适用于 Windows 10/11 x64，Dilmeter 1.6.3 及兼容版本；CN、RT、OT 共用同一包。普通玩家使用内置语音无需下载。

## 玩家操作

1. 将 ZIP 中的 `addons` 文件夹解压到软件 EXE 同级目录。
2. 确认文件位于 `软件目录/addons/edge-tts/python/python.exe`，不要多套一层包名目录。
3. 在“语音制作 → Edge 自然语音 · 晓晓”点击“自动监测”，即可生成、试听并保存。

无需安装 Python、pip、Edge 浏览器或 FFmpeg，无需修改环境变量。附加包只包含运行依赖；输入文字仍需发送给 Microsoft 在线服务以生成语音，已保存 MP3 可离线播放。分开目录的 CN、RT、OT 需要分别解压。关闭软件后删除 `addons/edge-tts` 可卸载，已保存音效不受影响。

## 实现与复现

- 从 `os.Executable()` 所在目录查找附加包，不以快捷方式的工作目录为准；找到可用附加包后优先使用。附加包缺失或不可用时继续检测已有系统 Python。
- 使用 Python 官方嵌入式运行时 3.13.16（x64）、edge-tts 7.3.1 和锁定的 Windows wheel。`python313._pth` 限制导入路径，不读取用户 Python 环境；不包含 pip，不安装到系统。
- `scripts/edge-tts-addon/lock.json` 固定每个组件的版本、URL、SHA-256。原始 wheel 的许可与 metadata 保留，edge-tts 的源码归档、LGPL 及 GPL 完整文本随包附带。
- 主程序通过子进程调用此包，文字经标准输入 JSON 传入，不拼接到 shell 命令。
- 普通更新器只替换更新 ZIP 中的文件，不主动删除独立的 `addons` 目录。

开发机使用 Python 3.10+，在仓库根目录运行：

```powershell
python scripts/package-edge-tts-addon.py
```

输出在 `artifacts/edge-tts-addon-v1.0.0/`，包括独立 ZIP 和文件校验清单。构建会验证官方输入哈希，并把最终 ZIP 解压到含中文和空格的目录，清空 PATH、设置无效的 PYTHONHOME/PYTHONPATH 后检查全部依赖导入、版本和 TLS 证书。

实际软件后端测试（先将 ZIP 解压到独立测试目录）：

```powershell
$env:DILMETER_EDGE_TTS_ADDON_TEST_ROOT = '绝对路径\解压目录'
$env:DILMETER_EDGE_TTS_NETWORK_TEST = '1'
cd cn-rt
go test ./cmd/dilmeterapi -run EdgeTTS -count=1 -v
go test -tags dilmeter_rt ./cmd/dilmeterapi -run EdgeTTS -count=1 -v
cd ../ot
go test ./cmd/dilmeterapi -run EdgeTTS -count=1 -v
```

网络测试发送固定的“绿幕即将出现”，检查生成有效 MP3；不设置 NETWORK_TEST 时只做离线依赖检测。CI 使用后一种方式，避免在线语音服务波动阻断普通构建。

## 发布与维护

本次仅准备本地附加包和软件接入，公开发布按 `docs/releasing.md` 执行。Release 工作流构建独立 ZIP 和校验清单；正式发布时将同一最终包同步至下载站和蓝奏云，下载页加可选下载入口。附加包独立版本为 1.0.0，在线服务接口变化时可单独更新此包。

参考：[Python 嵌入式发行版](https://docs.python.org/3.13/using/windows.html#the-embeddable-package)、[Python 3.13.16](https://www.python.org/downloads/release/python-31316/)、[edge-tts 项目](https://github.com/rany2/edge-tts)。

## 本地验证记录（2026-10-17）

- 附加包 `Dilmeter-EdgeTTS-Addon-v1.0.0-win-x64.zip`：12,704,272 字节（约 12.7 MB），SHA-256 `ee64a118106d43d49c63278f34be74671d9c044fa03802d9af4ef807962bb1b5`。
- 实际 ZIP 解压到中文、空格路径后，在 PATH 为空、PYTHONHOME/PYTHONPATH 指向不存在目录的条件下，Python 3.13.16、全部 14 个依赖、TLS 证书检查通过。
- CN、RT、OT 的后端附加包检测通过；CN 通过同一套调用生成“绿幕即将出现”，返回 12,096 字节的有效 MP3。没有进行扬声器听感验证。
- 三个后端全量测试通过；两个前端的类型检查、生产构建、职业 KPI、骑枪 KPI、战斗记录、Boss 提醒及队友模板回归通过，OT 台服资源和术语检查通过。保留现有构建分块提示。
- 本记录是本地构建验证；尚未推送源码、触发 Release 或将附加包上传到下载站/蓝奏云。
