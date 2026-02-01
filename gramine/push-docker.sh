#!/bin/bash
# push-docker.sh
# 推送 X Chain Docker 镜像到 GitHub Container Registry

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== 推送 X Chain Docker 镜像 ===${NC}"
echo ""

# 获取版本信息
VERSION="${1:-dev}"

# 镜像信息
REGISTRY="${DOCKER_REGISTRY:-ghcr.io}"
REPO_NAME="${DOCKER_REPO:-mccoysc/xchain-node}"
IMAGE_NAME="${REGISTRY}/${REPO_NAME}"

echo -e "${BLUE}推送配置:${NC}"
echo "  版本: ${VERSION}"
echo "  仓库: ${REGISTRY}"
echo "  镜像: ${IMAGE_NAME}:${VERSION}"
echo ""

# 步骤 1: 检查镜像是否存在
echo -e "${YELLOW}[1/4] 检查本地镜像...${NC}"
if ! docker images "${IMAGE_NAME}:${VERSION}" | grep -q "${VERSION}"; then
    echo -e "${RED}错误: 镜像 ${IMAGE_NAME}:${VERSION} 不存在${NC}"
    echo "请先构建镜像: ./build-docker.sh ${VERSION}"
    exit 1
fi
echo -e "${GREEN}✓ 找到本地镜像${NC}"

# 步骤 2: 登录到 GitHub Container Registry
echo -e "${YELLOW}[2/4] 登录到 GitHub Container Registry...${NC}"

# 检查是否已设置 GITHUB_TOKEN
if [ -z "${GITHUB_TOKEN}" ]; then
    echo -e "${YELLOW}未设置 GITHUB_TOKEN 环境变量${NC}"
    echo "请提供 GitHub Personal Access Token (需要 write:packages 权限)"
    read -sp "Token: " GITHUB_TOKEN
    echo ""
fi

echo "${GITHUB_TOKEN}" | docker login ghcr.io -u "${GITHUB_USER:-mccoysc}" --password-stdin

echo -e "${GREEN}✓ 登录成功${NC}"

# 步骤 3: 推送镜像
echo -e "${YELLOW}[3/4] 推送镜像...${NC}"

docker push "${IMAGE_NAME}:${VERSION}"

echo -e "${GREEN}✓ 镜像推送完成${NC}"

# 步骤 4: 推送 latest 标签（如果需要）
if [ "${VERSION}" != "dev" ] && [ "${VERSION}" != "test" ]; then
    read -p "是否标记并推送为 latest? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}[4/4] 推送 latest 标签...${NC}"
        docker tag "${IMAGE_NAME}:${VERSION}" "${IMAGE_NAME}:latest"
        docker push "${IMAGE_NAME}:latest"
        echo -e "${GREEN}✓ latest 标签推送完成${NC}"
    else
        echo "跳过 latest 标签"
    fi
else
    echo -e "${YELLOW}[4/4] 跳过 latest 标签（开发/测试版本）${NC}"
fi

echo ""
echo -e "${GREEN}=== 推送完成 ===${NC}"
echo ""
echo "镜像已发布到:"
echo "  ${IMAGE_NAME}:${VERSION}"
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "  ${IMAGE_NAME}:latest"
fi
echo ""
echo "拉取命令:"
echo "  docker pull ${IMAGE_NAME}:${VERSION}"
echo ""
echo "运行命令:"
echo "  # SGX 模式（需要 SGX 硬件）"
echo "  docker run -d --name xchain-node \\"
echo "    --device=/dev/sgx_enclave \\"
echo "    --device=/dev/sgx_provision \\"
echo "    -v /var/run/aesmd:/var/run/aesmd \\"
echo "    -v \$(pwd)/data:/data \\"
echo "    -p 8545:8545 -p 8546:8546 -p 30303:30303 \\"
echo "    ${IMAGE_NAME}:${VERSION} sgx"
echo ""
echo "  # Direct 模式（模拟器，无需 SGX）"
echo "  docker run -d --name xchain-node \\"
echo "    -v \$(pwd)/data:/data \\"
echo "    -p 8545:8545 -p 8546:8546 -p 30303:30303 \\"
echo "    ${IMAGE_NAME}:${VERSION} direct"
