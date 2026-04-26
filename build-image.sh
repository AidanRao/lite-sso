#!/bin/bash

# --- 配置部分 ---
IMAGE_NAME="lite-sso-server"

# 检查是否提供了版本号参数
if [ $# -eq 1 ]; then
  VERSION="$1"
  echo "Using provided version: ${VERSION}"
else
  VERSION=$(date +"%Y%m%d%H%M")
  echo "Using auto-generated version: ${VERSION}"
fi

# 定义所有需要推送的完整标签列表
# 我们直接在这里把本地标签和远程标签合并处理
REPOSITORIES=(
  "chenxuanrao/${IMAGE_NAME}:${VERSION}" # Docker Hub
  "crpi-cayqrvwyllekrteg.cn-beijing.personal.cr.aliyuncs.com/aidan_rao/${IMAGE_NAME}:${VERSION}" # Aliyun
)

# 动态构建 buildx 的标签参数
TAG_FLAGS=""
for TAG in "${REPOSITORIES[@]}"; do
  TAG_FLAGS="$TAG_FLAGS -t $TAG"
done

# --- 执行部分 ---

echo "🚀 开始多架构构建并直接推送到远程仓库..."
echo "目标平台: linux/amd64, linux/arm64"

# 核心步骤：使用 buildx 构建、打标并推送
# --push 会自动处理所有 -t 指定的标签
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  $TAG_FLAGS \
  --push \
  .

# 检查构建结果
if [ $? -eq 0 ]; then
  echo "------------------------------------------------"
  echo "✅ 所有镜像已成功构建并推送至："
  for TAG in "${REPOSITORIES[@]}"; do
    echo "  - $TAG"
  done
  echo "------------------------------------------------"
else
  echo "❌ 构建或推送过程中出错，请检查 Docker 指令输出。"
  exit 1
fi
