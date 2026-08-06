#!/usr/bin/env bash
# 在有网 / 可访问 ACR 的机器上执行：拉取镜像并导出为离线 tar
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGES_DIR="${ROOT_DIR}/images"
PLATFORM="${OFFLINE_PLATFORM:-linux/amd64}"
VERSION="$(tr -d '[:space:]' < "${ROOT_DIR}/VERSION")"
TAG="v${VERSION}"

# shellcheck disable=SC1091
if [[ -f "${ROOT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.env"
  set +a
fi
TAG="${WEKNORA_VERSION:-$TAG}"

REGISTRY="${ACR_REGISTRY:-registry.cn-beijing.aliyuncs.com}"
NAMESPACE="${ACR_NAMESPACE:-gov-claw}"

APP_IMAGES=(
  "${REGISTRY}/${NAMESPACE}/tree-rag-ui:${TAG}"
  "${REGISTRY}/${NAMESPACE}/tree-rag-app:${TAG}"
  "${REGISTRY}/${NAMESPACE}/tree-rag-docreader:${TAG}"
)
BASE_IMAGES=(
  "paradedb/paradedb:v0.22.2-pg17"
  "redis:7.0-alpine"
)

mkdir -p "${IMAGES_DIR}"

log() { printf '[pack] %s\n' "$*"; }
die() { printf '[pack][ERROR] %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"
}

need_cmd docker

if [[ -n "${ACR_USERNAME:-}" && -n "${ACR_PASSWORD:-}" ]]; then
  log "登录 ACR: ${REGISTRY}"
  echo "${ACR_PASSWORD}" | docker login "${REGISTRY}" \
    -u "${ACR_USERNAME}" --password-stdin
elif ! docker pull --platform "${PLATFORM}" \
  "${REGISTRY}/${NAMESPACE}/tree-rag-ui:${TAG}" >/dev/null 2>&1; then
  log "提示: 若 pull 失败，请先执行:"
  log "  export ACR_USERNAME=xxx ACR_PASSWORD=xxx"
  log "  或: docker login ${REGISTRY}"
fi

MANIFEST="${IMAGES_DIR}/manifest-${TAG}.txt"
: > "${MANIFEST}"

pull_and_save() {
  local image="$1"
  local safe
  safe="$(echo "${image}" | tr '/:' '__')"
  local out="${IMAGES_DIR}/${safe}.tar"
  log "拉取 ${PLATFORM} ← ${image}"
  docker pull --platform "${PLATFORM}" "${image}"
  log "导出 → ${out}"
  docker save -o "${out}" "${image}"
  echo "${image}  ${out}" >> "${MANIFEST}"
  ls -lh "${out}"
}

log "目标平台: ${PLATFORM}"
log "应用版本: ${TAG}"

for image in "${APP_IMAGES[@]}" "${BASE_IMAGES[@]}"; do
  pull_and_save "${image}"
done

# 可选：汇总为一个压缩包，便于拷贝
BUNDLE="${IMAGES_DIR}/treerag-images-${TAG}-${PLATFORM//\//-}.tar.gz"
log "打包镜像目录 → ${BUNDLE}"
tar -C "${IMAGES_DIR}" -czf "${BUNDLE}.tmp" \
  $(cd "${IMAGES_DIR}" && ls *.tar manifest-*.txt 2>/dev/null || true)
mv "${BUNDLE}.tmp" "${BUNDLE}"
ls -lh "${BUNDLE}"

log "完成。将整个 deploy/offline-intranet 目录拷到内网机后执行:"
log "  ./scripts/load-images.sh && ./scripts/deploy.sh"
