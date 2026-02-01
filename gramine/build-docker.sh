#!/bin/bash
# build-docker.sh
# 构建 X Chain Docker 镜像

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== 构建 X Chain Docker 镜像 ===${NC}"
echo ""

# 获取版本信息
VERSION="${1:-dev}"
BUILD_MODE="${2:-prod}"

# Git 信息
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# 镜像信息
REGISTRY="${DOCKER_REGISTRY:-ghcr.io}"
REPO_NAME="${DOCKER_REPO:-mccoysc/xchain-node}"
IMAGE_NAME="${REGISTRY}/${REPO_NAME}"

echo -e "${BLUE}配置信息:${NC}"
echo "  版本: ${VERSION}"
echo "  构建模式: ${BUILD_MODE}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  Git Branch: ${GIT_BRANCH}"
echo "  镜像名称: ${IMAGE_NAME}"
echo ""

echo -e "${BLUE}配置信息:${NC}"
echo "  版本: ${VERSION}"
echo "  构建模式: ${BUILD_MODE}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  Git Branch: ${GIT_BRANCH}"
echo "  镜像名称: ${IMAGE_NAME}"
echo ""
echo -e "${YELLOW}重要: 使用 Gramine 官方镜像环境编译，确保依赖一致性${NC}"
echo ""

# 步骤 1: 在 Gramine 环境中编译 geth（如果需要）
echo -e "${YELLOW}[1/5] 检查 geth 二进制...${NC}"
if [ ! -f "${REPO_ROOT}/build/bin/geth" ]; then
    echo -e "${YELLOW}geth 不存在，使用 Gramine 环境编译...${NC}"
    "${SCRIPT_DIR}/build-in-gramine.sh"
else
    echo -e "${YELLOW}geth 已存在，建议使用 Gramine 环境重新编译确保兼容性${NC}"
    read -p "是否在 Gramine 环境中重新编译? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        "${SCRIPT_DIR}/build-in-gramine.sh"
    else
        echo -e "${GREEN}✓ 使用现有 geth 二进制${NC}"
    fi
fi

# 步骤 2: 确保 manifest 模板存在
echo -e "${YELLOW}[2/5] 检查 Gramine 配置...${NC}"
if [ ! -f "${REPO_ROOT}/gramine/geth.manifest.template" ]; then
    echo -e "${RED}错误: Gramine manifest 模板不存在${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Gramine 配置文件存在${NC}"

# 步骤 3: 生成或复用签名密钥
echo -e "${YELLOW}[3/5] 准备签名密钥...${NC}"
if [ ! -f "${REPO_ROOT}/gramine/enclave-key.pem" ]; then
    echo -e "${YELLOW}签名密钥不存在，将在 Docker 构建时生成${NC}"
else
    echo -e "${GREEN}✓ 使用现有签名密钥${NC}"
fi

# 步骤 4: 构建 Docker 镜像
echo -e "${YELLOW}[4/5] 构建 Docker 镜像...${NC}"

cd "${REPO_ROOT}"

# Docker 构建（多阶段构建，在 Gramine 环境中编译）
docker build \
    -f Dockerfile.xchain \
    --build-arg VERSION="${VERSION}" \
    --build-arg BUILD_MODE="${BUILD_MODE}" \
    --build-arg GOVERNANCE_CONTRACT="0x0000000000000000000000000000000000001001" \
    --build-arg SECURITY_CONFIG_CONTRACT="0x0000000000000000000000000000000000001002" \
    -t "${IMAGE_NAME}:${VERSION}" \
    -t "${IMAGE_NAME}:${GIT_COMMIT}" \
    .

echo -e "${GREEN}✓ Docker 镜像构建完成（在 Gramine 环境中编译）${NC}"

# 步骤 5: 提取 MRENCLAVE
echo -e "${YELLOW}[5/5] 提取 MRENCLAVE...${NC}"
MRENCLAVE=$(docker run --rm "${IMAGE_NAME}:${VERSION}" cat /app/MRENCLAVE.txt 2>/dev/null || echo "unknown")
echo -e "${GREEN}MRENCLAVE: ${MRENCLAVE}${NC}"

# 保存到文件
echo "${MRENCLAVE}" > "${REPO_ROOT}/MRENCLAVE.txt"

echo ""
echo -e "${GREEN}=== 构建完成 ===${NC}"
echo ""
echo "镜像标签:"
echo "  - ${IMAGE_NAME}:${VERSION}"
echo "  - ${IMAGE_NAME}:${GIT_COMMIT}"
echo ""
echo "MRENCLAVE: ${MRENCLAVE}"
echo "（已保存到 MRENCLAVE.txt）"
echo ""
echo "下一步:"
echo "  1. 测试镜像: docker run -it ${IMAGE_NAME}:${VERSION} direct"
echo "  2. 推送镜像: ./push-docker.sh ${VERSION}"
echo "  3. 标记为 latest: docker tag ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}:latest"
