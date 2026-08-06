#!/usr/bin/env bash
# 安装 / 更新宿主机 Nginx 反代配置
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SRC="${ROOT_DIR}/nginx/weknora.conf"

log() { printf '[nginx] %s\n' "$*"; }
die() { printf '[nginx][ERROR] %s\n' "$*" >&2; exit 1; }

[[ -f "${SRC}" ]] || die "缺少 ${SRC}"
command -v nginx >/dev/null 2>&1 || die "未安装 nginx"

if [[ "$(id -u)" -ne 0 ]]; then
  SUDO=(sudo)
else
  SUDO=()
fi

if [[ -d /etc/nginx/sites-available ]]; then
  DEST_AVAIL=/etc/nginx/sites-available/weknora.conf
  DEST_ENABLED=/etc/nginx/sites-enabled/weknora.conf
  "${SUDO[@]}" cp "${SRC}" "${DEST_AVAIL}"
  "${SUDO[@]}" ln -sfn "${DEST_AVAIL}" "${DEST_ENABLED}"
elif [[ -d /etc/nginx/conf.d ]]; then
  DEST=/etc/nginx/conf.d/weknora.conf
  "${SUDO[@]}" cp "${SRC}" "${DEST}"
else
  die "未找到 /etc/nginx/sites-available 或 conf.d"
fi

"${SUDO[@]}" nginx -t
if command -v systemctl >/dev/null 2>&1; then
  "${SUDO[@]}" systemctl reload nginx
else
  "${SUDO[@]}" nginx -s reload
fi

log "已启用 weknora 反代 → 127.0.0.1:18080"
log "请按需修改 server_name / HTTPS 证书后 reload"
