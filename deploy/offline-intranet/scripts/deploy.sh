#!/usr/bin/env bash
# 内网机：启动服务并（可选）安装 Nginx 站点
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

log() { printf '[deploy] %s\n' "$*"; }
die() { printf '[deploy][ERROR] %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "未安装 docker"
docker compose version >/dev/null 2>&1 || die "未安装 docker compose"

[[ -f .env ]] || die "缺少 .env"
[[ -f docker-compose.yml ]] || die "缺少 docker-compose.yml"
[[ -f config/builtin_models.yaml ]] || die "缺少 config/builtin_models.yaml"

# shellcheck disable=SC1091
set -a
source .env
set +a

REQUIRED_IMAGES=(
  "registry.cn-beijing.aliyuncs.com/gov-claw/tree-rag-ui:${WEKNORA_VERSION}"
  "registry.cn-beijing.aliyuncs.com/gov-claw/tree-rag-app:${WEKNORA_VERSION}"
  "registry.cn-beijing.aliyuncs.com/gov-claw/tree-rag-docreader:${WEKNORA_VERSION}"
  "paradedb/paradedb:v0.22.2-pg17"
  "redis:7.0-alpine"
)

missing=0
for image in "${REQUIRED_IMAGES[@]}"; do
  if ! docker image inspect "${image}" >/dev/null 2>&1; then
    log "缺少镜像: ${image}"
    missing=1
  fi
done
if [[ "${missing}" -eq 1 ]]; then
  die "请先执行 ./scripts/load-images.sh"
fi

log "启动 compose ..."
docker compose up -d

log "等待 app 健康检查 ..."
for _ in $(seq 1 60); do
  if docker compose ps app 2>/dev/null | grep -qi healthy; then
    break
  fi
  sleep 5
done

log "服务状态:"
docker compose ps

log "检查内置模型 upsert 日志:"
docker compose logs app --tail 80 | grep -E 'Built-in model|builtin' || true

if [[ "${INSTALL_NGINX:-1}" == "1" ]]; then
  if [[ -x "${SCRIPT_DIR}/install-nginx.sh" ]]; then
    "${SCRIPT_DIR}/install-nginx.sh" || log "Nginx 安装跳过或失败（可稍后手动安装）"
  fi
fi

log "前端本机端口: http://127.0.0.1:${FRONTEND_PORT:-18080}"
log "经宿主机 Nginx 访问: http://<服务器IP>/"
log "完成"
