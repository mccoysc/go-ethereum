#!/bin/bash
# build-in-gramine.sh
# 在 Gramine 容器中编译 geth，确保与运行环境一致

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== 在 Gramine 容器中编译 geth ===${NC}"
echo ""
echo -e "${YELLOW}说明: 使用 Gramine 官方镜像编译，确保依赖一致性${NC}"
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

echo -e "${BLUE}步骤 1/3: 准备编译环境...${NC}"

# 创建临时 Dockerfile 用于编译
TEMP_DOCKERFILE="${REPO_ROOT}/.dockerfile-build-temp"
cat > "${TEMP_DOCKERFILE}" <<'EOF'
FROM gramineproject/gramine:latest

# 安装 Go 和构建工具
RUN apt-get update && apt-get install -y \
    wget \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# 安装 Go 1.21（添加重试和更好的错误处理）
RUN wget --timeout=30 --tries=3 https://go.dev/dl/go1.21.6.linux-amd64.tar.gz || \
    wget --timeout=30 --tries=3 https://golang.google.cn/dl/go1.21.6.linux-amd64.tar.gz || \
    (echo "Failed to download Go" && exit 1) && \
    tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz && \
    rm go1.21.6.linux-amd64.tar.gz

ENV PATH=/usr/local/go/bin:$PATH
ENV GOPATH=/go
ENV GO111MODULE=on

WORKDIR /build

# 编译命令将通过 volume 挂载源代码
CMD ["make", "geth"]
EOF

echo -e "${GREEN}✓ 编译环境配置完成${NC}"

echo -e "${BLUE}步骤 2/3: 构建编译镜像...${NC}"

# 构建编译镜像
docker build -f "${TEMP_DOCKERFILE}" -t xchain-builder:latest "${REPO_ROOT}"

echo -e "${GREEN}✓ 编译镜像构建完成${NC}"

echo -e "${BLUE}步骤 3/3: 在容器中编译 geth...${NC}"

# 在容器中编译
docker run --rm \
    -v "${REPO_ROOT}:/build" \
    -w /build \
    xchain-builder:latest \
    make geth

# 清理临时文件
rm -f "${TEMP_DOCKERFILE}"

echo ""
echo -e "${GREEN}=== 编译完成 ===${NC}"
echo ""
echo "编译输出: ${REPO_ROOT}/build/bin/geth"
echo ""

# 检查编译结果
if [ -f "${REPO_ROOT}/build/bin/geth" ]; then
    echo -e "${GREEN}✓ geth 编译成功${NC}"
    
    # 显示依赖信息
    echo ""
    echo -e "${BLUE}依赖库信息:${NC}"
    docker run --rm \
        -v "${REPO_ROOT}:/build" \
        xchain-builder:latest \
        ldd /build/build/bin/geth | head -10 || echo "无法获取依赖信息"
    
    echo ""
    echo -e "${YELLOW}下一步:${NC}"
    echo "  1. 本地测试: cd gramine && ./run-local.sh"
    echo "  2. Gramine direct: cd gramine && ./run-dev.sh direct"
    echo "  3. Gramine SGX: cd gramine && ./run-dev.sh sgx"
    echo "  4. Docker 构建: cd gramine && ./build-docker.sh"
else
    echo -e "${RED}✗ geth 编译失败${NC}"
    exit 1
fi
