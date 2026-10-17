Dilmeter 晓晓语音附加包 1.0.0

适用于 Dilmeter 1.6.3 及兼容版本，Windows 10/11 64 位。
CN、RT、OT 共用此包。它仅用于制作自定义晓晓语音，内置提示音不需要此包。

安装
1. 将压缩包里的 addons 文件夹解压到 Dilmeter EXE 所在目录。
   正确位置：软件目录\addons\edge-tts\python\python.exe
   不要多套一层 Dilmeter-EdgeTTS-Addon 文件夹。
2. 打开软件的“语音制作”，选择“Edge 自然语音 · 晓晓”，点击“重新检测”。
3. 显示“晓晓语音附加包已就绪”后，输入文字并生成，试听后保存提醒设定。

无需安装 Python 或 edge-tts，无需修改 PATH，无需管理员安装操作。
不需要安装或升级 Edge 浏览器，不需要 API Key 或 FFmpeg。
若几个版本的软件放在不同目录，请分别解压一份 addons。

联网与声音
本包提供语音制作依赖，不包含可离线合成的晓晓声音模型。
生成时会将输入文字发送到 Microsoft 在线语音服务；网络不通时无法生成。
已经生成并保存的 MP3 可离线播放，移除附加包后仍然可以使用。
在线服务失败时检查网络、系统时间和附加包版本，也可切换 Windows 本地语音。

更新与卸载
普通软件更新不会主动删除 addons 文件夹；移到新目录时请一起复制。
更新附加包时先关闭软件，删除旧的 addons\edge-tts 文件夹，再解压新包。
仅删除 addons\edge-tts 即可卸载，不要删除软件保存的自定义音效目录。
附加包内没有 pip；需要升级时换用新版附加包，已有系统 Python 的用户仍可自行安装。

组件与来源
Python 3.13.16 官方 Windows embeddable distribution:
https://www.python.org/downloads/release/python-31316/
edge-tts 7.3.1（第三方开源项目，非 Microsoft 官方发行的软件）:
https://github.com/rany2/edge-tts

第三方许可与源码
Python 许可在 addons\edge-tts\python\LICENSE.txt。
各依赖的原始许可保留在 python\Lib\site-packages 下对应的 .dist-info 目录。
edge-tts 的原始源码包、LGPL、GPL 许可位于 addons\edge-tts\sources 和 licenses。
组件版本、下载地址及原始文件 SHA-256 记录在 addons\edge-tts\bundle-lock.json。
edge-tts 源文件以未修改的 .py 形式随包分发，可直接查看或替换。
