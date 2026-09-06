# 来源与第三方许可

## 本项目的新贡献

本仓库中新编写和修改的部分适用根目录 [LICENSE](LICENSE) 中的 PolyForm Noncommercial 1.0.0。

Required Notice: Copyright 2026 liaogx (https://github.com/liaogx), for original contributions in this repository.

本许可不改变上游或依赖已经授予的权利。个人主页、推广内容和旧版功能宣传已移除；必要的来源和许可说明不属于可删除的推广署名。

## 上游来源

- 原始项目：[279458179/douyin-mcp](https://github.com/279458179/douyin-mcp)
- 审查与修改起点：`c1a4f16098a6db730bdc62dca25d5eacbd20155d`
- 该版本 README 的许可声明为 **“MIT License”**；该提交未附带独立 LICENSE 文件或明确版权年份。本项目保留该声明，不补造其权利人姓名或年份。
- 为便于分发，标准 MIT 许可文本保存在 [licenses/UPSTREAM-MIT.txt](licenses/UPSTREAM-MIT.txt)。其中适用于上游材料的权利归原权利人，不能将其重新声称为本项目独占或禁止其原本获准的商业使用。
- 上游 README 声明其架构参考 [xpzouying/xiaohongshu-mcp](https://github.com/xpzouying/xiaohongshu-mcp)。本次没有修改小红书项目，也没有将其代码作为依赖引入。

本版本重写了 HTTP 安全边界、浏览器生命周期、登录与 Cookie 存储、图文/视频发布流程，移除占位实现和短信/互动相关功能，补充测试与文档。以独立仓库发布，不表示此前材料没有来源。

## Go 依赖

依赖通过 Go modules 获取，没有将其源码直接复制为本项目源码。发布可执行文件或 Docker 镜像时仍须随附适用的版权、许可及 NOTICE；本仓库 `licenses/` 保存运行时依赖对应文件的逐字副本。

- `github.com/go-rod/rod` — MIT
- `github.com/go-rod/stealth` — MIT；其静态 evasions 脚本相关说明见该项目源码与许可证
- `github.com/modelcontextprotocol/go-sdk` — MIT
- `github.com/google/jsonschema-go` — BSD-3-Clause
- `github.com/segmentio/asm`、`github.com/segmentio/encoding` — MIT
- `github.com/yosida95/uritemplate/v3` — MIT
- `github.com/ysmood/*` — 以所附对应 LICENSE 为准
- `golang.org/x/*` — BSD-3-Clause

准确版本以 [go.mod](go.mod)、[go.sum](go.sum) 为准，副本列表见 [licenses/README.md](licenses/README.md)。PolyForm 文本来自 [PolyForm Project 官方许可文本](https://github.com/polyformproject/polyform-licenses/tree/1.0.0)，未修改条款正文。

Chrome/Chromium 由用户安装或发行版包管理器提供，适用其自身许可；容器内保留 Debian 软件包的版权文件。本项目的非商业许可不覆盖浏览器及其他第三方软件。
