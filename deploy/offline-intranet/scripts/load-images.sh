#!/usr/bin/env bash
# 内网机：从 images/ 导入全部镜像
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGES_DIR="${ROOT_DIR}/images"

log() { printf '[load] %s\n' "$*"; }
die() { printf '[load][ERROR] %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "未安装 docker"

[[ -d "${IMAGES_DIR}" ]] || die "目录不存在: ${IMAGES_DIR}"

shopt -s nullglob
tars=("${IMAGES_DIR}"/*.tar)
gz_bundles=("${IMAGES_DIR}"/treerag-images-*.tar.gz)

if ((${#tars[@]} == 0)) && ((${#gz_bundles[@]} > 0)); then
  log "发现汇总包，解压到 images/"
  for bundle in "${gz_bundles[@]}"; do
    tar -xzf "${bundle}" -C "${IMAGES_DIR}"
  done
  tars=("${IMAGES_DIR}"/*.tar)
fi

((${#tars[@]} > 0)) || die "images/ 下没有 *.tar，请先在有网机器运行 pack-images.sh 并拷贝过来"

for tarfile in "${tars[@]}"; do
  log "导入 $(basename "${tarfile}")"
  docker load -i "${tarfile}"
done

log "当前相关镜像:"
docker images --format 'table {{.Repository}}:{{.Tag}}\t{{.ID}}\t{{.Size}}' \
  | grep -E 'tree-rag|paradedb|redis|REPOSITORY' || true

log "完成"
