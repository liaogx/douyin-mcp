# douyin-mcp

**用自己的账号完成抖音登录、发布、搜索与单次授权互动。** 一个运行在你自己电脑上的 Go MCP 服务：通过创作者中心上传图文和视频，通过抖音网页版搜索、筛选、读取作品与评论，并操作点赞、收藏、楼中楼回复、@、表情和本地图片。MCP 与 REST API 共用同一套实现。

通过浏览器页面操作，默认在后台最小化运行，登录或安全验证时才恢复专用窗口供手动接管。不构造或重放逆向签名请求，不收集账号密码，不接管日常浏览器资料。提交时可被动读取当前页面发出的响应，用于确认业务结果。不提供短信验证码、自动破解验证、无限采集、群发互动、私信、交易或付费功能；不等于抖音平台的全部功能。

> 当前版本：`0.2.0-preview`。真实账号已验证扫码登录、专用浏览器重启保留登录、评论与楼中楼等交互；完整测试范围和未验收项目见 [验证记录](docs/verification.md)。**视频和图文作品的最终发布尚未通过真实账号完整验收**。页面版本、账号权限、实名要求和风控会影响使用；不是抖音官方 API，也不保证免验证或不封号。

## 主要功能

| MCP 工具 | 功能 | 行为说明 |
| --- | --- | --- |
| `check_login_status` | 检查登录状态 | `surface: creator` 用于发布；`surface: web` 用于搜索互动 |
| `get_login_qrcode` | 获取二维码 | 返回可展示的 PNG，只支持抖音 App 扫码 |
| `delete_cookies` | 清除本工具的登录状态 | 删除 Cookie 备份，销毁专用浏览器会话和待发布预览 |
| `publish_video` | 发布视频 | 本地文件上传、标题与正文填写、预览后确认提交 |
| `publish_image_text` | 发布图文 | 多图上传、标题与正文填写、预览后确认提交 |
| `get_search_filters` | 读取筛选条件 | 返回当前页面真正提供的筛选组、选项及选中值 |
| `search_posts` | 搜索与筛选作品 | 视频/图文、排序、发布时间、时长等；有限次数滚动 |
| `get_post_detail` | 作品详情 | 可见文案、计数、图片与可可靠识别的点赞/收藏状态 |
| `get_comments` | 评论与楼中楼 | 返回临时 `comment_ref`；传 `parent_ref` 读取子回复 |
| `set_post_like` | 点赞 / 取消赞 | `liked: true/false`，不盲目切换 |
| `set_post_favorite` | 收藏 / 取消收藏 | `favorited: true/false` |
| `comment_post` | 评论作品 | 文字、普通 emoji、平台表情、真实 @ 选择、本地单图 |
| `reply_comment` | 回复评论或子回复 | 用准确的 `comment_ref` 定位，不按昵称随便匹配 |
| `set_comment_like` | 评论点赞 / 取消赞 | `liked: true/false`，也适用于已加载的子回复 |
| `set_comment_dislike` | 评论点踩 / 撤销点踩 | `disliked: true/false`；裂开心形会折叠评论，不是取消赞 |
| `get_mention_candidates` | @ 候选人 | 返回昵称、头像、`mention_ref` 和页面可用的候选 ID |
| `get_emoji_options` | 平台表情列表 | 返回 `emoji_ref`、预览图片和可用标签；不捏造表情名称 |

共 **17 个 MCP 工具**。六个互动写入工具均先准备、再确认；候选人/表情查询不公开发送，但会替换未提交的互动预览。

### 安全默认值

- 默认仅监听 `127.0.0.1:18070`；MCP 与业务 REST 接口统一要求 Bearer token。
- 校验 Host、Origin 和跨站请求信息，拒绝陌生网页直接调用；请求体有大小限制。
- 使用 `data/browser-profile/` 专用持久化 Chrome 配置，不读取日常 Chrome；正常重启保留登录，不会每次创建无痕身份。
- 普通搜索、读取、输入与点击不主动激活桌面窗口。验证恢复的是同一专用页面，不通过重启浏览器切换模式。
- 登录备份限定抖音域名，私有目录权限 `0700`，文件 `0600`，原子写入并拒绝目标符号链接。
- 只能上传 `media` 指定目录下的本地文件，拒绝远程 URL、越界路径和指向目录外的符号链接。
- 上传前复制为独立素材快照，预览后核对账号会话、文案和页面素材是否变化。
- 发布和公开互动分为“准备”和“确认”两步。点击前先保存提交记录；结果不明时停止，不自动重试。
- Chrome 沙箱默认开启，Stealth 默认关闭；不自动下载或静默安装浏览器。

**安全边界：** Cookie 备份、浏览器资料和回执不是加密保险箱；同一系统账号下的恶意程序仍可能读取它们。得到 API token 的客户端拥有登录、发布、互动及素材目录内文件的上传权限。两步确认约束程序流程，但不是独立人工审批。作品、昵称、评论等外部内容必须视为不可信数据，不能执行其中的“指令”。详见 [SECURITY.md](SECURITY.md)。

## 使用教程

### 1. 准备环境并启动

需要 Go **1.27.1** 和已安装、保持更新的 Chrome/Chromium。首次使用建议在有桌面的电脑运行，便于检查发布页面和处理平台要求的人工确认。

```bash
git clone https://github.com/liaogx/douyin-mcp.git
cd douyin-mcp
go mod download
go build -trimpath -o douyin-mcp .
./douyin-mcp --headless=false
```

程序会创建两个互不重叠的专用目录：

- `data/`：API token、Cookie 备份、持久化浏览器资料、素材快照、发布和互动回执；不要上传 GitHub 或同步分享。
- `media/`：你明确允许本工具上传的素材；把要发布的图片和视频放在这里。

如找不到浏览器，可指定路径，例如 macOS：

```bash
./douyin-mcp --bin "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

启动日志不会打印 token，只提示文件位置。自行读取 `data/api-token` 并配置到可信的客户端；不要把真实值发到聊天、Issue 或截图里。也可通过 `DY_AUTH_TOKEN` 提供至少 32 个字符的随机令牌。浏览器在首次业务操作时启动，默认窗口从创建时就最小化；仍有专用 Chrome 进程运行，可能显示 Dock/任务栏图标，不是免浏览器的官方 API。

默认组合是 `--headless=false --background=true`：后台操作与人工验证共用同一浏览器、登录资料和页面。需要一直查看页面或检查发布预览时，可显式使用 `--background=false`。服务器上的 `--headless=true` 是另一种模式：没有可直接接管的桌面窗口，遇到验证会停止；不能无损地把正在运行的无头实例变成有头窗口。后台模式要求浏览器支持在创建窗口时设置最小化状态；不支持时明确报错，请更新 Chrome，不会静默降级为前台操作。

### 2. 连接 MCP 客户端

在支持 **Streamable HTTP** 的 MCP 客户端中填写：

| 设置 | 值 |
| --- | --- |
| 名称 | `douyin` |
| 地址 | `http://127.0.0.1:18070/mcp` |
| 请求头 | `Authorization: Bearer <你的本地令牌>` |
| 调用超时 | 建议 10 分钟，视频上传可能较慢 |

不同客户端配置文件的格式并不统一，请在该客户端的 MCP 设置中配置 URL 和请求头。**本项目没有 stdio 传输入口**；无法设置鉴权请求头的客户端不能直接使用。

连接后应发现上表中的 17 个工具。可向助手说明：

> 获取抖音网页版登录二维码，surface 设为 web。我扫码后检查状态；不要收集手机号、密码或验证码。写入互动先让我核对对象和内容。

### 3. 扫码登录

1. 搜索互动时调用 `get_login_qrcode({"surface":"web"})`；发布作品则使用 `{"surface":"creator"}`（默认值）。用自己的抖音 App 扫码确认。
2. 调用相同 `surface` 的 `check_login_status`。只有 `phase: "logged_in"` 且 `success: true` 才表示已确认登录。
3. 正常关闭再启动服务会沿用同一专用浏览器资料。网页版与创作者中心的登录资格分别检查，不因一个已登录就假定另一个也可发布。

不要手动删除 `data/browser-profile/`、改用另一数据目录、并行启动两个实例或调用 `delete_cookies` 后期待仍保持登录。旧版只有 `cookies.json` 的用户升级后需要一次扫码：新版本不把旧 JSON 覆盖回新的浏览器身份。Cookie 备份仍会保存，持久化资料是重启恢复的主要来源。

获取二维码后不会无限后台轮询，**扫码后务必再检查状态**。二维码过期时先查看专用页面刷新，重新调用；`delete_cookies` 是主动退出，会清除本工具全部本地登录资料，不应用来日常“刷新二维码”。

如果返回 `needs_attention`，在恢复的专用窗口或抖音 App 中自行完成平台要求的验证，并通知客户端继续。验证未解除时暂停其他操作；下一次调用先检查原页面，确认解除后重新最小化，不重新加载或自动重发。不要把另一个普通浏览器窗口的登录当作 MCP 登录。工具不会自动填写短信、绕过滑块或绕过实名要求。去掉短信接口不代表平台永远不会要求额外验证。

### 4. 准备图文或视频

视频准备参数：

```json
{
  "video_path": "我的视频.mp4",
  "title": "周末随拍",
  "description": "记录今天的日常。"
}
```

图文准备参数：

```json
{
  "image_paths": ["照片1.jpg", "照片2.png"],
  "title": "周末随拍",
  "description": "两张照片，一点记录。"
}
```

相对路径以 `media` 为根目录。绝对路径也必须位于配置的素材目录内。不同素材类型不能混用。

本工具的本地限制（**不代表抖音平台的官方限额**）：

- 标题：1–30 个 Unicode 字符，单行；正文：最多 1,000 个 Unicode 字符。
- 图文：1–35 张 JPG/JPEG、PNG 或 WebP，每张不超过 20 MiB。
- 视频：1 个 MP4/MOV，不超过 2 GiB；仅做容器特征检查，不保证编码、比例、时长符合平台要求。
- 最终允许的格式、数量及账号发布资格，以当前创作者中心为准。WebP/MOV 如被平台拒绝，请先自行转为其支持的格式。

准备步骤会向抖音上传素材、填写页面，返回 `stage: "ready"` 和 `draft_id`；**这时没有点击发布，`success` 为 `false` 是正常的**。准备也不承诺自动保存为平台草稿。请到专用浏览器核对账号、图片顺序、标题、正文及可见范围。

### 5. 明确确认后提交

得到本人对具体内容的确认后，再调用同一个发布工具，只传：

```json
{
  "draft_id": "上一步返回的32位标识",
  "confirm": true
}
```

确认时不要重新传标题或路径。想改内容，应重新准备并确认新的 `draft_id`。

| 返回状态 | 含义 | 应如何处理 |
| --- | --- | --- |
| `ready` | 页面准备完成，尚未发布 | 核对内容，再确认 |
| `submitted` + `success: true` | 页面出现明确的提交成功反馈 | 仍需等待平台审核；重复确认只返回已有回执 |
| `unknown` | 已尝试点击，但无法确认结果 | **先去创作者中心检查，不要让助手重试发布** |

每次只保留一个待确认预览，30 分钟内有效。新的准备会替换旧预览；服务重启、清除会话后未提交的预览失效。提交回执可跨重启读取，但不是一个完整的任务队列，也不承诺全平台“绝对只发一次”：人为重新准备同一内容仍可能导致重复作品。

目前不支持定时发布、自动设置可见范围、允许评论/合拍开关、封面、音乐、话题选择器或多账号。平台页面的既有默认值会保留，提交前请自行核对。

### 6. 搜索与过滤帖子

先调用 `get_search_filters({"query":"猫咪日常"})`，再从返回值中选择条件。例如当前页面提供以下选项时：

```json
{
  "query": "猫咪日常",
  "tab": "general",
  "filters": [
    {"group":"排序依据","option":"最新发布"},
    {"group":"内容形式","option":"图文"}
  ],
  "limit": 10,
  "max_scrolls": 1
}
```

将上述参数传给 `search_posts`。每次最多返回 50 条，最多额外滚动 5 次；不是全量搜索导出。平台不同页面/账号的条件可能不同，不支持的选项会报错，不会悄悄忽略。可以按排序、发布时间、时长、内容形式、搜索范围筛选，但必须以实时返回的选项为准。

首次搜索通过抖音主页的真实输入框和“搜索”按钮触发，后续关键词/筛选变更复用同一个已加载的标签页，不直接拼接带关键词的地址。每选择一项筛选，都等待结果稳定后再选下一项；菜单临时关闭时重新悬停展开，读取前后复核全部选中条件。“相关搜索”推荐词卡片不计入作品。

页面显示“服务出现异常”或服务器/网络异常时，搜索最多通过原页面的“搜索”按钮恢复一次，并重新设置、核验全部原筛选；仍失败会报出异常及触发的筛选步骤，**不会把异常当成零条结果，也不会偷偷取消筛选返回默认结果**。人工点击搜索同样可能清空平台筛选；恢复看到内容不等于原条件已生效。人工验证、登录异常、超时及评论/点赞等写操作不走这条重试路径；遇到人工验证会保留当前页面，完成验证后用原参数继续。

筛选选中不等于平台检索结果必然准确。仍须打开详情复核发布时间、内容和可见互动数据；网页没有提供的“最多评论”“同城”等选项不能假装已设置。“还未看过”有助于减少重复，但不能替代自己的历史互动记录。

拿到 `posts[].url` 后传给 `get_post_detail` 或 `get_comments`。可用作品数字 ID、完整 `/video/`、`/note/` 地址或带 `modal_id` 的搜索地址；不接受短链、外站、用户主页或任意网页地址。工具会绑定具体作品，避免误操作搜索流里预加载的下一条视频。

新详情页会等待目标文案区域和互动控件稳定，避免在图文页面刚出现外壳、控件尚未挂载时返回不完整信息。目标改变仍立即停止；页面改版或加载失败不是可以跳过绑定检查的理由。

### 7. 点赞、收藏、评论和楼中楼

准备点赞，调用 `set_post_like`：

```json
{"post":"作品ID或完整地址","liked":true}
```

准备评论，调用 `comment_post`：

```json
{"post":"作品ID或完整地址","text":"不错 😊"}
```

返回 `stage: "ready"` 后，核对具体作品、账号、正文和目标，再调用**同一个工具**：

```json
{"action_id":"返回的32位标识","confirm":true}
```

`completed` + `success:true` 表示当前页面已确认操作结果；`unchanged` 表示本来就是所需状态，没有多点一次。`unknown` 表示可能已经发出：先用 `get_comments` 或人工检查，**即使刚完成平台身份验证，也不要直接重发**。审核、折叠、排序和刷新会影响后续可见性，不能把页面提交成功当成公开可见的保证。

其他互动参数：

| 工具 | 准备字段（另加 `post`） |
| --- | --- |
| `set_post_like` | `liked: true/false` |
| `set_post_favorite` | `favorited: true/false` |
| `reply_comment` | `comment_ref`、`text` 或图片等内容 |
| `set_comment_like` | `comment_ref`、`liked: true/false` |
| `set_comment_dislike` | `comment_ref`、`disliked: true/false` |

楼中楼流程：`get_comments` 取得父评论引用 → 同一工具加 `parent_ref` 读取子回复 → `reply_comment` 传所选子回复的 `comment_ref`。会校验作者、正文、图片、父级关系和页面标识；不会只靠重复昵称定位。`comment_ref` 是本服务的临时引用，不是平台永久评论 ID；30 分钟后、服务重启或评论内容变化后需重新读取。点踩折叠会改变正文，撤销前也应重新读取引用。

### 同页确认与低流量工作流

同一作品的详情、评论准备/发送、点赞准备/确认复用当前详情页；客户端应按作品逐条完成已授权操作，再切换下一条，不必在每个步骤后重新打开链接。成功回执保存在本地，去重优先读本地记录；**只有评论已确认成功且点赞也获得授权时，才继续点赞**。确认阶段仍只点击一次，不合并授权、不自动批量执行。

- 普通顶层文字评论优先被动观察本次页面 POST：核对目标作品和正文，要求 HTTP 200、业务 `status_code: 0`、新评论 ID，以及响应中的账号/作品/正文一致。接口适配采用严格白名单；200、空响应、验证码页面或业务错误均不能单独证明成功。
- 无可识别响应时，评论可在**同一页面**核对新增的本人评论及正文/父级关系。已观察到业务失败或响应冲突时，不用乐观回显覆盖错误；不自动刷新、另开页或重发。楼中楼、图片、@ 和平台表情目前仍按页面证据确认。
- 作品点赞/收藏必须观察到与本次设置值相符的业务成功响应，并核对当前控件状态；不再仅凭瞬时变色报成功。响应缺失、格式改变或超时返回 `unknown`，不额外开页探测或自动重试。评论赞/点踩仍是页面状态确认，不宣称服务端持久化已验收。
- 回执新增 `verification`（`platform_response` / `visible_comment` / `page_state`）；能可靠取得时附 `platform_comment_id`。这些是确认依据，不保证平台审核通过、其他账号可见或永久保存。
- 响应监听仅在单次提交期间启用，限定当前页面、来源和动作；不保存原始请求/响应、签名参数或请求头，不额外发送后台写请求。严格适配未覆盖的页面版本会明确报不确定，不能把模拟测试当成真实站点全部兼容。
- 检测到平台验证时，保留当前专用页面（包括准备失败时的验证页面），尝试激活对应标签页、恢复最小化窗口；交由用户手动验证，不点短信发送、不填验证码。操作系统可能限制前台焦点；默认后台模式无需重启即可接管，显式无头模式则没有可恢复的桌面窗口。验证完成后不自动重发 `unknown` 操作，仍须先核验结果。

### 8. @、平台表情和本地图片

1. `get_mention_candidates({"post":"作品ID","query":"昵称"})`：先看候选昵称和头像，选择具体 `mention_ref`。同名候选不按位置猜测；没有平台 ID 时仅返回实际可见资料。
2. `get_emoji_options({"post":"作品ID"})`：选择 `emoji_ref`。有些表情没有文字标签，用返回的图片核对；普通 Unicode emoji 可直接放在 `text` 中。
3. 将选中的引用加入 `comment_post` 或 `reply_comment` 的准备参数：

```json
{
  "post": "作品ID",
  "comment_ref": "从get_comments取得的引用，仅回复时需要",
  "text": "不错",
  "image_path": "我的图片.png",
  "mentions": ["选中的mention_ref"],
  "emojis": ["选中的emoji_ref"]
}
```

`comment_post` 请省略 `comment_ref`。普通文本“@昵称”不等于平台真实提及；工具会实际打开面板、选择候选，再核对编辑器。上述引用只对同一作品有效，30 分钟过期。先取齐候选，再准备最终内容；查询候选或切换作品可能使待确认互动失效。

本地限制：正文最多 500 字，最多 5 个 @、10 个平台表情、1 张 20 MiB 以内 JPG/PNG/WebP。图片必须在素材根目录内。**准备阶段就会上传图片给抖音，但尚未公开发送评论**；平台账号权限和当前页面仍可能拒绝图片功能。出现歧义或上传异常时停止，不降级为悄悄发送纯文字。

### REST API

健康检查不需要令牌，不暴露账号状态：

```bash
curl http://127.0.0.1:18070/health
```

以下示例假设你已在本地终端设置 `DY_AUTH_TOKEN` 为实际令牌。不要把示例占位值当成可用密码，也不要将它提交到代码仓库。

```bash
# 检查登录
curl -H "Authorization: Bearer ${DY_AUTH_TOKEN}" \
  http://127.0.0.1:18070/api/v1/login/status

# 获取二维码；qr_image 是 PNG data URL，可由本地界面展示
curl -X POST -H "Authorization: Bearer ${DY_AUTH_TOKEN}" \
  http://127.0.0.1:18070/api/v1/login/qrcode

# 准备视频，不立即发布
curl -X POST http://127.0.0.1:18070/api/v1/publish/video \
  -H "Authorization: Bearer ${DY_AUTH_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"video_path":"我的视频.mp4","title":"周末随拍","description":"记录今天的日常。"}'

# 确认提交：替换为上一步的真实 draft_id
curl -X POST http://127.0.0.1:18070/api/v1/publish/video \
  -H "Authorization: Bearer ${DY_AUTH_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"draft_id":"替换为返回的32位标识","confirm":true}'

# 清除本工具的登录状态
curl -X DELETE -H "Authorization: Bearer ${DY_AUTH_TOKEN}" \
  http://127.0.0.1:18070/api/v1/cookies
```

图文接口是 `POST /api/v1/publish/image-text`。网页版登录检查为 `GET /api/v1/login/status?surface=web`；二维码接口 POST JSON `{"surface":"web"}`。省略 surface 保留创作者中心默认行为。

新增业务接口均为 POST，要求同一 Bearer token 与 JSON 请求体：

| REST 路径 | 对应 MCP 工具 |
| --- | --- |
| `/api/v1/web/search/filters` | `get_search_filters` |
| `/api/v1/web/search` | `search_posts` |
| `/api/v1/web/post` | `get_post_detail` |
| `/api/v1/web/comments` | `get_comments` |
| `/api/v1/web/mentions` | `get_mention_candidates` |
| `/api/v1/web/emojis` | `get_emoji_options` |
| `/api/v1/web/<工具名>` | 上表六个互动写入工具，工具名原样填写 |

REST 参数与 MCP 相同，严格拒绝未知参数。没有短信验证码接口；发布参数中的定时、权限等未实现字段仍然拒绝，不会假装已应用。

### 配置选项

| 参数 | 环境变量 | 默认值 / 说明 |
| --- | --- | --- |
| `--host` | `DY_HOST` | `127.0.0.1`，不建议直接暴露外网 |
| `--port` | `DY_PORT` | `18070` |
| `--bin` | `ROD_BROWSER_BIN` | 查找已安装的 Chrome/Chromium |
| `--data-dir` | `DY_DATA_DIR` | `./data`，专用私有目录 |
| `--media-root` | `DY_MEDIA_ROOT` | `./media`，允许上传的素材目录 |
| `--headless` | `DY_HEADLESS` | `false`，保留可人工接管的桌面浏览器能力；设为 `true` 则没有可恢复的桌面窗口 |
| `--background` | `DY_BACKGROUND` | `true`，专用窗口默认最小化，登录/验证时恢复；`headless=true` 时忽略 |
| `--stealth` | `DY_STEALTH` | `false`，可选兼容脚本 |
| `--proxy` | `DY_PROXY` | 无；只支持不含账号密码的 HTTP/HTTPS/SOCKS5 URL |
| `--user-agent` | `DY_USER_AGENT` | 空，保留浏览器真实默认值 |
| `--no-sandbox` | `DY_NO_SANDBOX` | `false`，不要轻易关闭浏览器沙箱 |
| `--timeout` | — | `10m`，允许 `30s`–`30m` |
| — | `DY_AUTH_TOKEN` | 自动生成并保存在 `data/api-token`，也可提供随机值 |

命令行覆盖对应环境变量。旧版 `COOKIES_PATH` 不再使用；请用 `--data-dir`。不要把数据目录或素材目录设置成主目录、项目根目录，也不能互相包含。

### Docker

提供非 root 用户的容器配置和 amd64/arm64 构建入口，使用发行版 Chromium；**本次开发环境没有 Docker，容器构建及浏览器沙箱仍待实测**。首次账号验收优先本地有界面运行。

```bash
mkdir -p media
docker compose -f docker/docker-compose.yml up -d --build
```

详见 [Docker 使用说明](docker/README.md)。不要暴露浏览器调试端口，不要使用 `--privileged`，不要挂载日常 Chrome 配置或 Docker socket。

## 技术架构

```text
可信 MCP 客户端 / REST 客户端
              │ Bearer token + Host/Origin 校验
              ▼
       HTTP 入口（标准库）
              │ 单账号串行执行、超时与关闭处理
              ▼
  登录 / 发布状态机 / 网页搜索互动服务
              │ 作品、账号、评论、正文和素材绑定
              │ 准备 → 确认一次 → 持久化回执
              ▼
    独立 Chrome/Chromium 会话
              │ 持久化专用资料；不接管日常 Chrome
              ▼
      抖音网页版 / 抖音创作者中心
```

主要目录：`browser/` 管浏览器生命周期，`cookies/` 管凭证格式，`douyin/` 管网页选择器和发布流程，`internal/securefile/` 管私有文件。`service.go` 串行协调操作；`security.go` 负责 HTTP 访问控制；`mcp_handlers.go` 与 `routes.go` 分别提供 MCP 和 REST 入口。

发布适配集中在 `douyin/page.go`、`douyin/login.go`、`douyin/publish_page.go`；网页版适配在 `douyin/web_*.go`。`web_service.go` 复用串行执行器，`web_handlers.go` 注册 MCP/REST。互动回执位于 `data/interaction-receipts/`，不保存 Cookie 值。选择器不确定、目标变化或多个匹配控件时停止，不盲目点击。

`browser/interaction.go` 为点击、输入和二维码截图提供独立的 10 秒上限，较短的请求期限优先；使用真实浏览器输入并检查遮挡，不依赖后台标签页可能暂停的动画帧。页面事件监听保留到标签页关闭，避免一次请求结束就损坏登录页或后续导航。

### 技术栈与版本

版本核对日期：**2026-09-06**。锁定版本见 [go.mod](go.mod)、[go.sum](go.sum)，不是运行时自动升级。

| 组件 | 本项目使用版本 | 用途 |
| --- | --- | --- |
| Go | `1.27.1` | 服务、文件边界保护、HTTP |
| MCP Go SDK | `github.com/modelcontextprotocol/go-sdk v1.7.0` | 官方 MCP 实现 |
| MCP 协议 | `2026-07-28`；兼容 `2025-11-25` 初始化流程 | Stateless Streamable HTTP；由 SDK 协商 |
| go-rod | `v0.116.2` | Chrome DevTools Protocol 自动化 |
| go-rod/stealth | `v0.4.9` | 可选的静态浏览器兼容脚本 |
| HTTP 框架 | Go 标准库 `net/http` | 不再依赖 Gin |
| Chrome/Chromium | 用户安装的当前稳定版 | 不内置浏览器版本；应持续更新 |
| Docker 构建基础 | Go `1.27.1`、Debian 13 | 容器 Chromium 版本以构建时软件源为准 |

直接依赖使用核对时最新稳定发布版。传递依赖以兼容性和漏洞检查为前提更新；`github.com/ysmood/fetchup` 保留 `v0.2.3`：较新的 `v0.5.3` 修改了 API，当前 rod 无法编译。程序明确指定本地浏览器，**不走 fetchup 的浏览器自动下载流程**。不要直接无审查执行全量升级后声称测试通过。

`github.com/ysmood/got` 同样保留兼容的 `v0.41.0`：`v0.43.0` 已移除当前 rod 使用的 `lib/lcs`。这两项升级都实际触发了编译失败，因此没有强行采用不兼容版本。

参考：[Go 发布列表](https://go.dev/dl/)、[MCP SDK v1.7.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)、[rod](https://github.com/go-rod/rod)、[stealth](https://github.com/go-rod/stealth)。

### Stealth 模式是什么意思？

Stealth 会注入一段静态脚本，对部分常见自动化特征（例如 `navigator.webdriver`、插件信息等）做兼容处理，减少页面因自动化环境而出现的差异。

它**不是隐身、防盗号或防封号功能**：不会隐藏 IP，不保证躲过平台检测，也不解决验证码、实名、限流或违反平台规则的问题。默认关闭；仅在了解风险、遵守平台规则的前提下按需启用。

## 开发、测试与后续方向

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...

# 显式启用本地 Chrome 模拟页面测试；不向抖音发送作品
DY_BROWSER_TESTS=1 go test -race ./... -count=1

# 可选：访问真实抖音匿名登录页；不扫码、不发布
DY_LIVE_LOGIN_TEST=1 go test ./douyin -run TestLiveAnonymousQRCode -v
```

测试范围和验收缺口见 [测试记录](docs/verification.md)。所有测试素材动态生成，不包含真实 Cookie、个人照片或账户信息。

后续优先顺序：真实创作者发布验收；覆盖更多图文/视频页面版本；增加本地二维码与强人工确认界面；提供只读的晚到结果核对和更精确的作品/评论回执。多账号、定时、私信、关注、转发、删除历史内容与批量任务当前不实现；添加它们需要独立授权和验证，不应笼统声称“平台所有交互都支持”。

## 许可

本项目新编写和修改的代码采用 **[PolyForm Noncommercial 1.0.0](LICENSE)**，属于**源码公开、非商业许可**，不是 MIT/Apache，也不应称为 OSI 定义的开源软件。

允许符合完整条款的非商业使用、研究、修改和再分发。许可不授予商业用途授权，例如收费代运营、收费代发或作为商业营销服务的一部分；边界情况应结合完整条款判断，中文摘要不替代许可证。分发时须保留许可证与必要声明。

Required Notice: Copyright 2026 liaogx (https://github.com/liaogx), for original contributions in this repository.

这是一个独立发布的修改版本，不是抖音官方项目。上游和第三方组件仍适用各自许可证，不能被本项目的“非商业”条款追溯覆盖；来源与必要声明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
