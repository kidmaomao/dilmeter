# OT 外置资源包

DilmeterOT 会优先读取程序旁 `data/resource-pack` 目录中的资源；DilmeterCN 与 DilmeterRT 始终使用随程序发布的国服数据。

1. 在 Prilus 资源页下载目标服务器的 `*_resourcedata.bin.br` 与 `*_resourceversion.json`。
2. 保持服务器目录结构放入资源包目录。例如台服使用：

   - `data/resource-pack/resourcedata/tw/tw_resourcedata.bin.br`
   - `data/resource-pack/resourceversion/tw/tw_resourceversion.json`

3. 新建 `data/resource-pack/active-region.txt`，内容只写地区代码，例如 `tw`。
4. 重新启动 DilmeterOT。可用地区代码为 `kr`、`krt`、`cn`、`jp`、`tw`、`us`。

也可以不使用 `active-region.txt`，直接以应用请求路径覆盖文件：

- `data/resource-pack/resourcedata/cn/cn_resourcedata.bin.br`
- `data/resource-pack/resourceversion/cn/cn_resourceversion.json`

删除外置文件或 `active-region.txt` 后，OT 自动回退到内置国服资源。
