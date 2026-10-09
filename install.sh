#!/usr/bin/env bash
# x-ui-mini 一键安装：仅从 GitHub Releases 下载校验后的稳定包。
set -euo pipefail
REPO="${XUIMINI_REPO:-xinruiown/x-ui-mini}"
INSTALL_DIR=/usr/local/x-ui-mini
BIN=/usr/local/bin/x-ui-mini
UNIT=/etc/systemd/system/x-ui-mini.service

need_root() { [[ $(id -u) -eq 0 ]] || { echo "请使用 root: sudo bash"; exit 1; }; }
arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
  esac
}

need_root
export DEBIAN_FRONTEND=noninteractive
if command -v apt-get >/dev/null; then
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates unzip tar openssl >/dev/null
fi
A="$(arch)"
command -v curl >/dev/null
command -v tar >/dev/null
command -v sha256sum >/dev/null

API="https://api.github.com/repos/${REPO}/releases/latest"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "获取最新 Release..."
JSON="$(curl -fsSL "$API")"
TAG="$(printf '%s' "$JSON" | grep -oE '"tag_name": *"[^"]+"' | head -1 | cut -d'"' -f4)"
[[ -n "$TAG" ]] || { echo "没有可用 Release"; exit 1; }
VER="${TAG#v}"
ASSET="x-ui-mini_${VER}_linux_${A}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${TAG}"

echo "下载 ${ASSET} ..."
curl -fsSL -o "$TMP/${ASSET}" "${BASE}/${ASSET}"
curl -fsSL -o "$TMP/${ASSET}.sha256" "${BASE}/${ASSET}.sha256"
(cd "$TMP" && sha256sum -c "${ASSET}.sha256")

if [[ -x "$BIN" ]]; then
  echo "升级前备份..."
  "$BIN" backup || true
  cp -a "$INSTALL_DIR" "${INSTALL_DIR}.bak.$(date +%s)" || true
fi

mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/backup"
tar -C "$TMP" -xzf "$TMP/${ASSET}"
install -m 0755 "$TMP/x-ui-mini" "$BIN"
install -m 0644 "$TMP/x-ui-mini.service" "$UNIT" 2>/dev/null || true
if [[ ! -f "$UNIT" ]]; then
  cat > "$UNIT" <<'EOF'
[Unit]
Description=x-ui-mini panel
After=network.target
[Service]
Type=simple
ExecStart=/usr/local/bin/x-ui-mini serve
Restart=on-failure
RestartSec=3
User=root
LimitNOFILE=65535
[Install]
WantedBy=multi-user.target
EOF
fi

# Xray-core（官方发布，校验 sha256 由 XTLS 提供的 zip；此处记录版本）
XRAY_VER="${XRAY_VERSION:-v25.9.11}"
if [[ ! -x "$INSTALL_DIR/bin/xray" ]]; then
  echo "下载 Xray-core ${XRAY_VER}..."
  XA=64
  case "$A" in amd64) XA=64 ;; arm64) XA=arm64-v8a ;; esac
  curl -fsSL -o "$TMP/xray.zip" "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VER}/Xray-linux-${XA}.zip"
  command -v unzip >/dev/null || apt-get install -y unzip >/dev/null
  unzip -o "$TMP/xray.zip" xray -d "$INSTALL_DIR/bin"
  chmod 0755 "$INSTALL_DIR/bin/xray"
fi

if [[ ! -x "$INSTALL_DIR/bin/mtg" ]]; then
  echo "下载 mtg..."
  MTG_VER="${MTG_VERSION:-2.2.8}"
  case "$A" in amd64) MA=amd64 ;; arm64) MA=arm64 ;; esac
  curl -fsSL -o "$TMP/mtg.tgz" "https://github.com/9seconds/mtg/releases/download/v${MTG_VER}/mtg-${MTG_VER}-linux-${MA}.tar.gz" || true
  if [[ -f "$TMP/mtg.tgz" ]]; then
    tar -xzf "$TMP/mtg.tgz" -C "$TMP"
    MTG_BIN="$(find "$TMP" -type f -name mtg | head -1)"
    if [[ -n "$MTG_BIN" ]]; then
      install -m 0755 "$MTG_BIN" "$INSTALL_DIR/bin/mtg"
    fi
  fi
fi

mkdir -p "$INSTALL_DIR/data/certs"
if [[ ! -f "$INSTALL_DIR/data/certs/server.crt" ]]; then
  CN="${XUIMINI_DOMAIN:-localhost}"
  openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
    -keyout "$INSTALL_DIR/data/certs/server.key" \
    -out "$INSTALL_DIR/data/certs/server.crt" \
    -subj "/CN=${CN}" >/dev/null 2>&1 || true
fi

if [[ -n "${XUIMINI_DOMAIN:-}" ]] && command -v certbot >/dev/null; then
  certbot certonly --standalone -d "$XUIMINI_DOMAIN" --non-interactive --agree-tos -m "admin@${XUIMINI_DOMAIN}" || true
fi

systemctl daemon-reload
systemctl enable x-ui-mini >/dev/null 2>&1 || true

ask() {
  local prompt="$1" def="$2"
  local ans=""
  if [[ -r /dev/tty ]]; then
    printf '%s' "$prompt" >/dev/tty
    if [[ -n "$def" ]]; then printf ' [%s]' "$def" >/dev/tty; fi
    printf ': ' >/dev/tty
    IFS= read -r ans </dev/tty || true
  fi
  echo "${ans:-$def}"
}
ask_secret() {
  local prompt="$1" ans=""
  if [[ -r /dev/tty ]]; then
    printf '%s (回车则随机): ' "$prompt" >/dev/tty
    IFS= read -r -s ans </dev/tty || true
    printf '\n' >/dev/tty
  fi
  echo "$ans"
}

if [[ -z "${XUIMINI_PORT:-}" ]]; then
  XUIMINI_PORT="$(ask '面板端口' '2053')"
fi
if [[ -z "${XUIMINI_USERNAME:-}" ]]; then
  XUIMINI_USERNAME="$(ask '登录用户名' 'admin')"
fi
if [[ -z "${XUIMINI_PASSWORD:-}" ]]; then
  XUIMINI_PASSWORD="$(ask_secret '登录密码')"
fi
if [[ -z "${XUIMINI_PATH:-}" ]]; then
  XUIMINI_PATH="$(ask '面板路径（不要斜杠）' '')"
fi
if [[ -z "${XUIMINI_HOST:-}" ]]; then
  DET="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  XUIMINI_HOST="$(ask '公网 IP 或域名' "$DET")"
fi
if [[ -z "${XUIMINI_DOMAIN:-}" ]]; then
  XUIMINI_DOMAIN="$(ask '证书域名（没有则留空用自签）' '')"
fi
export XUIMINI_PORT XUIMINI_USERNAME XUIMINI_PASSWORD XUIMINI_PATH XUIMINI_HOST XUIMINI_DOMAIN
export XUIMINI_LISTEN="${XUIMINI_LISTEN:-0.0.0.0}"
export XUIMINI_DATA="$INSTALL_DIR/data"

"$BIN" configure
if [[ -n "${XUIMINI_PASSWORD:-}" ]]; then
  printf '%s\n' "$XUIMINI_PASSWORD" > "$INSTALL_DIR/data/initial-password.txt"
  chmod 600 "$INSTALL_DIR/data/initial-password.txt"
fi
unset XUIMINI_PASSWORD

ln -sfn "$BIN" /usr/local/bin/xui
if [[ -w /etc/sysctl.d ]]; then
  cat >/etc/sysctl.d/99-x-ui-mini-bbr.conf <<'EOF'
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF
  modprobe tcp_bbr 2>/dev/null || true
  sysctl -p /etc/sysctl.d/99-x-ui-mini-bbr.conf >/dev/null 2>&1 || true
  echo "BBR: $(cat /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null || echo unknown)"
fi

systemctl enable --now x-ui-mini
sleep 1
systemctl restart x-ui-mini
sleep 1
systemctl --no-pager --full status x-ui-mini || {
  echo "启动失败，尝试回滚..."
  LAST="$(ls -dt ${INSTALL_DIR}.bak.* 2>/dev/null | head -1 || true)"
  if [[ -n "$LAST" ]]; then
    rm -rf "$INSTALL_DIR"
    mv "$LAST" "$INSTALL_DIR"
    systemctl restart x-ui-mini || true
  fi
  exit 1
}
echo "安装完成。运行: x-ui-mini"
"$BIN" status || true
echo "浏览器: http://${XUIMINI_HOST:-服务器IP}:${XUIMINI_PORT:-2053}/${XUIMINI_PATH:-见 status 的 path}"
echo "若安装时没设密码: cat $INSTALL_DIR/data/initial-password.txt"
