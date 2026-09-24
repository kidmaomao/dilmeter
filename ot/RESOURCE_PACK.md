# OT 外置资源包说明

DilmeterOT 会优先读取程序旁 `data/resource-pack` 目录中的资源；DilmeterCN 与 DilmeterRT 始终使用随程序发布的国服数据。

1. 在 [Prilus 资源页](https://prilus.gitlab.io/) 下载目标服务器的 `*_resourcedata.bin.br` 与 `*_resourceversion.json`，两份文件使用同一服务器的版本。
2. 保持服务器目录结构放入资源包目录。例如台服使用：

   - `data/resource-pack/resourcedata/tw/tw_resourcedata.bin.br`
   - `data/resource-pack/resourceversion/tw/tw_resourceversion.json`

3. 新建 `data/resource-pack/active-region.txt`，内容只写地区代码，例如 `tw`。
4. 重新启动 DilmeterOT。可用地区代码为 `kr`、`krt`、`cn`、`jp`、`tw`、`us`。

也可以不使用 `active-region.txt`，直接以应用请求路径覆盖文件：

- `data/resource-pack/resourcedata/cn/cn_resourcedata.bin.br`
- `data/resource-pack/resourceversion/cn/cn_resourceversion.json`

例如台服目录如下：

```text
DilmeterOT-v1.5.1.exe
data/
  resource-pack/
    active-region.txt                 内容：tw
    resourcedata/tw/tw_resourcedata.bin.br
    resourceversion/tw/tw_resourceversion.json
```

关闭 OT 后，将整个 `data/resource-pack` 目录移走或重命名，再启动即可恢复内置国服资料。仅删除 `active-region.txt` 时，若目录中仍有直接覆盖的 `cn` 文件，它们仍会生效。

外置文件准备好后，读取资料不依赖资料站联网。当前入口覆盖的是资料数据和版本信息，并不提供完整外服图标包的一键切换；程序仍带有默认国服配套图标。未内置的图标可能需要联网读取。

资源包不会改变 OT 的服务器地址、端口或抓包协议支持范围。CN、RT 不读取此目录。
