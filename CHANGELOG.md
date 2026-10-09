# Changelog

## v0.1.0 — 2026-10-09

- 初始 MVP：Go 面板 + 内嵌中文 WebUI，默认仅监听 127.0.0.1:2053。
- 支持创建 VLESS-Reality-Vision、Shadowsocks、Trojan 入站配置并生成分享链接。
- SOCKS5 默认绑定本机；任意门转发；MTProto 链接生成。
- install.sh 从 GitHub Releases 校验下载；升级前备份，失败尝试回滚。
- GitHub Actions 构建 linux/amd64 与 linux/arm64。
- 未把密钥上传到仓库。
