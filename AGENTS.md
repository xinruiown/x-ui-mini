# x-ui-mini · HR / WorkBuddy / WebCodex 多 AI 协作

工作前阅读 `README.md`、`CHANGELOG.md`、`.ai/STATE.md`、`.ai/HANDOFF.md`。

- 源码：https://github.com/xinruiown/x-ui-mini（public）。必须先 `git pull --ff-only`，不要 `git push --force`。
- 一键安装地址：https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh；发行包：https://github.com/xinruiown/x-ui-mini/releases/latest。运行部署脚本前人工核对脚本内容与目标服务器身份。
- 当前发行基线 v0.2.2，后续需核对 GitHub Release 与源码一致；更改功能时更新 `CHANGELOG.md` 与 `.ai/HANDOFF.md`，注明测试、回滚。
- 现有 VMiss/3X-UI/代理节点属于运行环境，禁止直接修改或泄漏其真实 IP、密钥、Reality 私钥、服务器密码、证书、客户端订阅。
- 构建验证：`go test ./...`；GitHub Actions 构建 amd64/arm64 并生成哈希；发布正式版本必须用 git tag 触发 Release。
- 升级前确认实际可用备份，因服务运行涉及公网代理，拒绝静默覆盖；失败时用已验证版本回滚。
- 任何修复优先“修改→测试→记录→发布”，不做持续无效 A/B。
