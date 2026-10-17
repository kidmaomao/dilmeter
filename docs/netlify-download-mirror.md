# Netlify 下载页镜像

- 线上地址：https://noginogi-downloads.netlify.app/downloads/
- 来源：https://gear.noginogi.sbs/downloads/
- 独立项目：`noginogi-downloads`，团队 `kidmaomao`。
- Site ID：`417f8316-8d53-48cf-bd10-664f86b9bbd2`。
- 首次生产部署：`6ad3987db77bb46491b15d30`（2026-10-17）。
- 当前生产部署：`6ad3b1590e474edac87e9ad7`（2026-10-18，同步 1.6.3、晓晓附加包与四个蓝奏云入口）。
- 本地发布目录：`artifacts/netlify-download-mirror`；严格限定 9 个页面、样式、脚本、图标和路由文件。没有 ZIP、代理函数或私有日志。

## 更新

先依照 releasing.md 完成主站及正式安装包同步。使用 Python 3.9+ 执行：

```powershell
python scripts/build-download-mirror.py
```

在 `artifacts/netlify-download-mirror` 中使用已登录的官方 CLI 部署：

```powershell
npx --yes netlify-cli deploy --prod --dir . --site 417f8316-8d53-48cf-bd10-664f86b9bbd2 --no-build --json
```

项目未连接 Git 持续部署，使用显式 Site ID，不需要本地链接状态。`.netlify` 已加入忽略规则；发布目录禁止包含任何额外文件。

构建脚本从已发布的 gear 页面取快照，将 ZIP 地址改成 gear 绝对地址，并保留 GitHub 最新版检查及蓝奏云备用入口。非下载页导航指向 gear；遗留 ZIP 路径仅以 302 跳转 gear，不代理流量。主站尚未发布 1.6.3 时，镜像保持已核实的 1.6.2，不能预先添加不存在的附加包地址。

部署后从未登录窗口打开 `/downloads/`，确认状态 200、三个版本一致、gear/GitHub/蓝奏云均指向公开链接，Netlify 上无二进制文件。首次上线已完成页面和链接检查；未以大陆三网测速替代普通可访问性检查。

## Cloudflare 更新包方案（已搁置）

2026-10-18：按用户决定不接入 Cloudflare。以下保留为历史评估记录，当前下载方式仍为 gear / GitHub / 蓝奏云。

建议评估 R2 Standard + 自定义下载域名，作为补充更新源。R2 免出站流量费，免费额度每月 10 GB-month 存储、100 万次 A 类操作和 1,000 万次 B 类操作。超过额度后存储 $0.015/GB-month，A 类 $4.50/百万次，读取 $0.36/百万次。使用 Workers 或额外服务时另计；用量按官方规则向上取整。来源：[R2 定价](https://developers.cloudflare.com/r2/pricing/)（2026-10-17 核实）。

例如约 20 MB 的包下载 1 万次约为 200 GB；在账户存储和请求总量均处于免费额度、没有额外付费服务的前提下，R2 费用可为 $0。生产使用自定义域名，`r2.dev` 为限速的开发入口：[公共存储桶](https://developers.cloudflare.com/r2/buckets/public-buckets/)。

普通 R2 不等于大陆加速线路；官方 China Network 有单独的产品可用范围与接入条件。大陆三网速度需实测后再决定是否作主源，保留 gear、GitHub、蓝奏云备用。来源：[China Network 产品可用性](https://developers.cloudflare.com/china-network/reference/available-products/)。
