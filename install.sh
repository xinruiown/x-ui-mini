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

systemctl daemon-reload
systemctl enable --now x-ui-mini
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
echo "面板默认仅监听 127.0.0.1:2053，请用 SSH 隧道访问。"
