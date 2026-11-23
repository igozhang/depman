# etc 目录

| 文件 | 说明 |
|------|------|
| `depman.conf` | 运行参数。头部为生效配置，尾部为说明与示例。缺失或无效时启动会自动生成默认文件。`title` 为页眉品牌名（兼浏览器标题）。slogan 固定为 `practice makes perfect`（双击复制超管 igokey）。 |
| `icon.svg` / `icon.png` | **可选**站点 icon。有则作为 favicon 与页眉图标；无则使用内置 CICD SVG。 |
| `banner.jpg` 等 | **可选**自有头部 banner（jpg/png/webp/gif）。 |

写操作 igokey 活跃密钥在 `<data_dir>/igokey-active.json`（不在 etc）。
凭据加密密钥在 `<data_dir>/secret.key`。

容器部署时通常将 `DEPMAN_ETC_DIR` 指到 PVC 上的 `/data/etc`。
