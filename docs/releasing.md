# 正式发布流程

正式发布必须完成 GitHub Release、Gitee Release、NogiNogi 下载页、蓝奏云及 Netlify 下载页镜像同步。仅推送 main 或上传单一来源不算发布完成。

1. 核对 CN、RT、OT 的功能范围、版本和 Windows 文件属性。通用修复必须检查 OT 是否同步，并保留 OT 的语言和资源差异。
2. 准备更新公告；用户要求先确认公告时，使用用户最后确认的文字后再发布。两个 `release-notes-current.txt`、ZIP 内公告、三个更新清单和下载页公告应一致。
3. 执行后端测试、前端类型检查、职业 KPI（含 `verify:lancer-kpi`）、战斗记录及本次改动相关回归；OT 还需通过台服资源和术语检查。
4. 推送源码到 main，通过 `Release Dilmeter` 工作流发布版本。手动触发时使用 main 和不带 v 的版本号，避免同时推送标签导致重复构建。
5. 工作流完成后下载最终的三个 ZIP 和三个 JSON，以 GitHub Release 最终产物为准，核对版本、文件大小、SHA-256 及 ZIP 内容。后续镜像必须使用同一批文件，不能混用本地或另一轮构建产物。
6. 打开 [蓝奏云管理页](https://pc.woozooo.com/mydisk.php)，在已登录账户中新建 `Dilmeter_v1_6_2` 形式的版本目录，上传最终的 `DilmeterCN.zip`、`DilmeterRT.zip`、`DilmeterOT.zip`。不要覆盖旧版本目录；不要将源码、日志或用户配置上传。
7. 获取三个文件的公开分享链接，以及版本文件夹的分享链接和提取密码（若有）。从公开分享页核实版本、文件名和大小，并至少校验一个实际下载文件的 SHA-256。若需要登录或验证码，由用户完成相应步骤，不能将上传未完成记录为已发布。
8. 在 `gearscan/downloads/` 同步 ZIP 和 JSON。站内 JSON 的 `url` 指向站内 ZIP，大小、SHA-256 与 GitHub 相同。每个版本卡片添加独立的“蓝奏云备用下载”链接及版本标记；有密码时在链接旁明示。管理页 `mydisk.php` 不能作为下载链接。
9. 执行 `npm run verify:dilmeter-downloads`，检查公告、版本、大小、哈希、蓝奏云链接及新版本出现时隐藏旧版网盘入口的行为。将下载站变更推送 main，等待部署成功，再核实 [线上下载页](https://gear.noginogi.sbs/downloads/) 和公开下载。
10. 发布结果记录源码提交、Release、下载站部署及蓝奏云分享链接。网盘上传或站点部署失败时明确说明未完成项，修复后补齐。

蓝奏云分享链接仅用于用户手动备用下载，不可写入客户端更新清单的 `url`，因为分享页不是 ZIP 直链。不得在仓库、工作流或发布记录中保存账户 Cookie、密码或其他登录凭证。

## GitHub 与 Gitee 双端发布（1.6.4 起）

- 正式工作流发布 GitHub 产物后，运行 `scripts/sync-gitee-release.py`，把同一批最终 CN、RT、OT ZIP 同步到公开仓库 `kidmao100/dilmeter-releases`。仅同步本次版本，不补传历史版本。
- 令牌只从 GitHub Actions 加密 Secret `GITEE_ACCESS_TOKEN` 或本地 `--token-file` 读取；不可写入源码、下载清单、命令行参数或日志。令牌文件必须留在仓库外。
- 脚本先核验三个 ZIP 的版本、大小、SHA-256 和公告，再创建预发布、上传附件；三个附件均通过免登录实际下载及哈希核对后，才转为正式版并更新 `master/latest/DilmeterCN.json`、`DilmeterRT.json`、`DilmeterOT.json`。清单必须填写 Gitee 返回的公开 ZIP 下载地址。
- Gitee 失败时不得宣称双端发布完成。使用 GitHub 最终产物重试同一脚本；禁止重新构建另一批 ZIP 冒充同一版本。脚本复用已有附件并验证，不覆盖同名文件。
- 客户端提供自动、GitHub 优先、Gitee 优先。自动模式依次尝试 GitHub、Gitee、下载站；检查清单超时、下载失败或持续过慢会换源。备用源的版本、大小、SHA-256 必须与已选更新一致，避免下载尚未同步的旧包。
- 发布记录包含 Gitee 版本页、免登录下载验证结果及三个公开更新清单。首次升级到 1.6.4 后才启用新的来源选择和自动换源逻辑。

## Netlify 下载页镜像

镜像地址为 https://noginogi-downloads.netlify.app/downloads/ ，只展示页面与 gear / GitHub / 蓝奏云链接，不托管或代理安装包。每次正式版的主站下载页部署并核实后，执行 `scripts/build-download-mirror.py`，再按 [镜像维护记录](netlify-download-mirror.md) 上传页面并检查版本、链接和公开访问。不得将整个 artifacts、仓库、安装包、抓包或登录信息作为发布目录。

## 可选晓晓语音附加包（1.6.3 起）

- `scripts/package-edge-tts-addon.py` 按 `scripts/edge-tts-addon/lock.json` 构建独立的 Windows x64 ZIP；Release 工作流同时上传附加包及其大小/SHA-256 清单，不混入 CN、RT、OT 主包。
- 正式发布时同样下载 GitHub Release 的最终附加包和清单，核验后同步到下载站和本次蓝奏云目录，下载页标注“可选：自定义晓晓语音制作”“Windows 10/11 64 位”“软件 1.6.3 起支持”。下载地址和蓝奏云链接须核实后填写，不能预先添加不存在的链接。
- 保留包内使用说明、第三方许可、edge-tts 原始源码和组件锁定清单。使用附加包的实际解压目录运行 `TestEdgeTTSAddonIntegration`；常规构建验证不依赖 Microsoft 在线服务，发布前另外执行联网生成检查。
- 更新依赖时修改锁定版本、官方下载地址和 SHA-256，并提升独立的 `addonVersion`。客户端更新不会自动更新附加包，玩家关闭软件后替换 `addons/edge-tts` 即可。
