# x-ui-mini

超轻量中文节点面板。操作风格参考 [vaxilu/x-ui](https://github.com/vaxilu/x-ui)，**源码独立实现（MIT）**，不是该仓库的 fork，因此不整仓复用其 GPL-3.0 代码。

面向 128MB～1GB Debian VPS，AMD64 / ARM64。不依赖 Docker、Node.js、Nginx 或大型数据库。默认管理后台只监听 `127.0.0.1`。

现有 VMISS / 3X-UI 生产环境与本项目隔离，安装不会改动它们。

## 一键安装

仓库需先有 GitHub Release。当前账号：

```bash
curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash
```

安装脚本从 Releases 下载并 `sha256sum` 校验二进制，失败回滚上一份目录备份。

```bash
x-ui-mini            # 中文菜单
x-ui-mini status
x-ui-mini backup
x-ui-mini restore <file>
x-ui-mini update     # 提示再次运行 install.sh
```

访问面板：`ssh -L 2053:127.0.0.1:2053 user@vps` 后打开 `http://127.0.0.1:2053/<path>`。

默认用户 `admin`，首次密码见数据目录 `initial-password.txt`（v0.1.0 若文件不存在，请在 Web 登录前用 `XUIMINI_DATA` 下的 config 重置）。

## v0.1.0 已实现

- 一键创建 VLESS-Reality-Vision、Shadowsocks、Trojan（Trojan 入站 TLS 证书需在高级/后续版本挂载，当前生成链接）
- SOCKS5 落地，默认 `127.0.0.1`
- TCP 任意门（dokodemo-door），模式标记 relay/ingress/egress
- Telegram MTProto：生成 `tg://` 与 secret（独立 mtg 进程为后续模块）
- 分享链接；二维码在 v0.1.0 Web 以链接文本为主，后续补内嵌码
- systemd、备份恢复、升级回滚脚本
- AmneziaWG / Hysteria2：标记为按需模块，v0.1.0 不默认安装

## 安全

- 不要把 `config.json`、密码、Reality 私钥、证书私钥提交到 Git
- 管理端口不要映射到公网，除非你明确改 listen 并加防火墙

## 许可证

MIT。Xray-core 为其自身许可证，由安装脚本另行下载。
