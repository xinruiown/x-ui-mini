# x-ui-mini · GitHub 管理与安装入口

## GitHub 地址
- 仓库：https://github.com/xinruiown/x-ui-mini
- 已核实最新 Release（2026-10-21 检查时）：https://github.com/xinruiown/x-ui-mini/releases/tag/v0.2.2
- 以后获取最新正式版本：https://github.com/xinruiown/x-ui-mini/releases/latest
- 一键安装脚本：https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh

## 一键安装
在你自己有权限的 Linux VPS 上执行，**先审核安装脚本**：

```bash
curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash
```

它依赖 GitHub Releases 可用，安装时可能安装/升级 Xray、mtg、systemd 服务。不是在 Windows/软路由本机直接运行。会提示密码、监听路径、端口；请在升级现有 VPS 前备份并核实端口冲突。

## AI 交接（Hermes HR / WorkBuddy）
1. 进入 `D:\AI-Workspace\x-ui-mini`，先读 `AGENTS.md` 和 `.ai/HANDOFF.md`。
2. `git pull --ff-only` 后做改动；运行 `go test ./...`、必要的隔离安装验证；严禁把服务器认证信息提交 Git。
3. 更新 `CHANGELOG.md` 和 `.ai/{STATE,TASKS,HANDOFF}.md`，将风险、迁移和回滚说明写明。
4. `git add` 指定文件，`git commit`，`git push origin main`。不能强推主分支；重要功能按 `vX.Y.Z` tag 触发 Release。
5. 保持线上生产独立；没有服务器侧授权不得直接部署。GitHub 备份的是代码，不是 VPS 运行数据库和账号信息。
