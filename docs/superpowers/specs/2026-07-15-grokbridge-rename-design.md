# GrokBridge 完整重命名设计

## 目标

将项目从 `Grok2API` 完整重命名为 `GrokBridge`，并将 Git 远程仓库绑定到 `https://github.com/werbenhu/grokbridge.git`。本次变更采用破坏式重命名，不保留旧名称的兼容入口。

## 命名规则

- 展示名称：`Grok2API` → `GrokBridge`
- 小写标识：`grok2api` → `grokbridge`
- 大写标识：`GROK2API` → `GROKBRIDGE`
- Go module：`github.com/chenyme/grok2api/backend` → `github.com/werbenhu/grokbridge/backend`
- GitHub 仓库及镜像引用：迁移到 `werbenhu/grokbridge`

## 变更范围

更新所有受版本控制的项目标识，包括：

- README、前后端页面标题、Swagger 标题和仓库链接
- Go module、Go import 路径和命令入口目录
- 前端包名、运行时变量及静态资源文件名
- 可执行文件名、Makefile、Windows 构建和启动脚本
- Docker Compose 项目、服务、容器、镜像、卷和运行用户
- Dockerfile 缓存、路径、入口点和二进制名称
- 配置示例、环境变量、Redis 键前缀、数据库示例名和安全发行者标识
- 测试中的项目专属工具名前缀、请求头及断言
- GitHub Actions、Issue 模板和仓库相关元数据

Git 历史和许可证正文不重写。用户当前未提交的 Windows 支持文件及 `.gitignore`、`.gitattributes` 变更将保留、同步改名并纳入最终实现提交。

## 路径与兼容性

需要时将 `backend/cmd/grok2api` 等项目专属路径重命名为 `grokbridge`，静态资源也改用新文件名。旧环境变量、旧二进制名、旧 Docker 服务名、旧 Redis 前缀和旧工具名前缀不提供兼容别名；已有部署需按新名称调整配置。

## 验证

实现后执行以下验证：

1. 全仓扫描，确认受版本控制文件中不再出现 `Grok2API`、`grok2api`、`GROK2API`、`chenyme/grok2api`。
2. 运行 Go 格式检查和完整后端测试。
3. 运行前端 lint、类型检查或生产构建，以项目现有脚本为准。
4. 检查 Docker Compose 配置可解析。
5. 检查 Git diff、远程地址和提交内容，确保用户既有改动未丢失。

## 提交结果

实际重命名完成并验证后，将全部实现变更和用户现有未提交文件纳入一个实现提交；`origin` 设置为 `https://github.com/werbenhu/grokbridge.git`。除非用户另行要求，本任务只创建本地提交，不自动推送。
