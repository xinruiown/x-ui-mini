# Changelog

## v0.2.2

- 安装时可设置端口、用户名、密码、路径、公网地址、证书域名。
- 服务器输入 `xui` 打开中文管理菜单（启停、改密、BBR、备份、卸载等）。
- 安装时尝试开启 BBR。
- SOCKS5 / HTTP 可设账号密码；Telegram 使用 mtg；任意门中文模式说明。
- 面板可用 IP:端口/路径 直连，分享链接写入真实公网地址。

## v0.1.0 — 2026-10-09

- 初始 MVP：Go 面板 + 内嵌中文 WebUI，默认仅监听 127.0.0.1:2053。
- 支持创建 VLESS-Reality-Vision、Shadowsocks、Trojan 入站配置并生成分享链接。
- SOCKS5 默认绑定本机；任意门转发；MTProto 链接生成。
- install.sh 从 GitHub Releases 校验下载；升级前备份，失败尝试回滚。
- GitHub Actions 构建 linux/amd64 与 linux/arm64。
- 未把密钥上传到仓库。
