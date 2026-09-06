# Docker 部署

此配置为预览版，当前尚未在 Docker 环境实测。首次真实账号验收建议使用有桌面的本地启动方式。

在项目根目录运行：

```bash
mkdir -p media
docker compose -f docker/docker-compose.yml up -d --build
docker compose -f docker/docker-compose.yml logs --tail=30
```

令牌在私有数据卷中，不会出现在日志。仅在自己的终端读取并填入可信 MCP 客户端：

```bash
docker compose -f docker/docker-compose.yml exec douyin-mcp \
  sh -c 'cat /app/data/api-token'
```

不要复制真实令牌到 Issue、仓库或聊天。服务地址 `http://127.0.0.1:18070/mcp`，请求头为 `Authorization: Bearer <本地令牌>`。

把要上传的文件放入宿主机 `media/`。容器的路径以 `/app/media` 为根，调用时建议只给相对路径。素材挂载为只读，程序将其复制到私有数据卷的临时目录后上传。

## 安全与环境限制

- 容器内是 UID/GID `10001` 的非 root 用户；宿主机素材需要允许该用户读取，但不要把敏感目录整体改为公开可读。
- 只绑定宿主机回环地址，未发布 Chrome 调试端口。API token 同时保护 REST 与 MCP。
- Chromium 沙箱保持开启。若宿主机不允许非特权用户命名空间，浏览器可能无法启动；优先修复隔离环境，**不要默认使用 privileged、SYS_ADMIN 或关闭沙箱**。
- `DY_NO_SANDBOX=true` 仅是显式的高风险兼容选项，不代表推荐配置。
- 默认无头模式，二维码通过 MCP 返回。平台如要求额外的人机验证，停止自动流程，改用本地有界面方式处理；镜像没有远程桌面。
- 私有数据卷包含 Cookie、token、专用浏览器资料与发布/互动回执。停止容器保留登录资料；重新构建不会自动退出账号。主动退出使用 `delete_cookies`；不要删除持久化卷后期待仍保留登录，也不要与宿主机同时使用同一浏览器资料。
- 健康检查仅说明 HTTP 服务存活，不代表已登录、浏览器正常或发布功能通过验收。

## 多架构构建

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -f docker/Dockerfile -t douyin-mcp:preview .
```

是否可输出为本地多架构镜像取决于 Docker builder 的配置。使用 Go `1.27.1-trixie` 构建和 Debian 13 运行时；Chromium 来自 Debian 软件源，重建时获得该源更新。维护者应同时验证镜像和 Chromium 版本，不能只升级 Go。
