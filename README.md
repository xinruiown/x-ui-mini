# x-ui-mini

超轻量中文节点面板。操作风格参考 [vaxilu/x-ui](https://github.com/vaxilu/x-ui)，**源码独立实现（MIT）**，不是该仓库的 fork。

面向 128MB～1GB Debian VPS，AMD64 / ARM64。不依赖 Docker、Node.js、Nginx 或大型数据库。

现有 VMISS / 3X-UI 生产环境与本项目隔离，安装不会改动它们。

## 一键安装

在服务器终端执行（会询问端口、用户名、密码、路径、公网地址、证书域名；回车用默认/随机）：

```bash
curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash
```

也可用环境变量跳过问答：

```bash
sudo XUIMINI_USERNAME=admin XUIMINI_PORT=2053 XUIMINI_PATH=panel \
  XUIMINI_PASSWORD='你的密码' XUIMINI_HOST='你的IP或域名' bash -c \
  'curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | bash'
```

安装脚本从 Releases 校验下载二进制，并安装 Xray、mtg、自签证书，尝试开启 BBR。

```bash
xui                 # 中文管理菜单（改密、改端口、启停、BBR、备份、卸载）
x-ui-mini status
x-ui-mini bbr
x-ui-mini backup
x-ui-mini restore <file>
```

浏览器打开：`http://公网IP:端口/路径`  
默认用户 `admin`，密码见 `/usr/local/x-ui-mini/data/initial-password.txt`（若安装时自己设了密码，以你设的为准）。

## 已实现

- 一键创建 VLESS-Reality-Vision、Shadowsocks、Trojan（自签或 Let’s Encrypt）
- SOCKS5 / HTTP 落地，可设用户名密码
- TCP 任意门：中转 / 入口 / 落地
- Telegram MTProto（mtg）
- 分享链接；Web 可改节点名称并复制链接
- systemd、备份恢复、升级回滚
- AmneziaWG / Hysteria2：按需，默认不装

## 安全

- 不要把 `config.json`、密码、Reality 私钥、证书私钥提交到 Git
- SOCKS/HTTP 若对公网开放必须设密码

## 许可证

MIT。Xray-core、mtg 为其自身许可证，由安装脚本另行下载。
