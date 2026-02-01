#!/bin/bash
# check-environment.sh
# 检查当前编译和运行环境是否正确配置

set -e

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== 环境检查 ===${NC}"
echo ""

# 检查是否在 Gramine 容器中
echo -e "${BLUE}检查运行环境:${NC}"
if [ -f /etc/gramine_version ]; then
    echo -e "${GREEN}  ✓ 在 Gramine 容器中${NC}"
    cat /etc/gramine_version
    IN_GRAMINE=true
elif command -v gramine-sgx &> /dev/null; then
    echo -e "${GREEN}  ✓ Gramine 已安装（宿主机）${NC}"
    gramine-sgx --version
    IN_GRAMINE=false
else
    echo -e "${YELLOW}  ⚠ Gramine 未安装（宿主机环境）${NC}"
    IN_GRAMINE=false
fi
echo ""

# 检查 Docker
echo -e "${BLUE}检查 Docker:${NC}"
if command -v docker &> /dev/null; then
    echo -e "${GREEN}  ✓ Docker 已安装${NC}"
    docker --version
else
    echo -e "${RED}  ✗ Docker 未安装${NC}"
    exit 1
fi
echo ""

# 检查 Go
echo -e "${BLUE}检查 Go:${NC}"
if command -v go &> /dev/null; then
    echo -e "${GREEN}  ✓ Go 已安装${NC}"
    go version
else
    echo -e "${RED}  ✗ Go 未安装${NC}"
    exit 1
fi
echo ""

# 检查 SGX 设备（可选）
echo -e "${BLUE}检查 SGX 设备（可选）:${NC}"
if [ -c /dev/sgx_enclave ]; then
    echo -e "${GREEN}  ✓ SGX enclave 设备可用${NC}"
    SGX_AVAILABLE=true
else
    echo -e "${YELLOW}  ⚠ SGX enclave 设备不可用（开发测试可选）${NC}"
    SGX_AVAILABLE=false
fi

if [ -c /dev/sgx_provision ]; then
    echo -e "${GREEN}  ✓ SGX provision 设备可用${NC}"
else
    echo -e "${YELLOW}  ⚠ SGX provision 设备不可用（开发测试可选）${NC}"
fi
echo ""

# 建议
echo -e "${BLUE}=== 环境建议 ===${NC}"
echo ""

if [ "$IN_GRAMINE" = true ]; then
    echo -e "${GREEN}当前环境适合:${NC}"
    echo "  ✅ 生产编译"
    echo "  ✅ Manifest 生成和签名"
    echo "  ✅ 完整集成测试"
    if [ "$SGX_AVAILABLE" = true ]; then
        echo "  ✅ SGX 真实环境测试"
    fi
else
    echo -e "${YELLOW}当前环境适合:${NC}"
    echo "  ✅ 代码开发和验证"
    echo "  ✅ 快速语法检查"
    echo "  ✅ Go 单元测试"
    echo ""
    echo -e "${YELLOW}建议:${NC}"
    echo "  • 生产编译请使用: ${BLUE}./build-in-gramine.sh${NC}"
    echo "  • 这会在 Gramine 容器中编译以确保一致性"
fi
echo ""

# 推荐的工作流程
echo -e "${BLUE}=== 推荐工作流程 ===${NC}"
echo ""
echo "1. 开发阶段（当前环境）:"
echo "   ${BLUE}go fmt ./...${NC}"
echo "   ${BLUE}go vet ./...${NC}"
echo "   ${BLUE}go build ./...${NC}"
echo ""
echo "2. 编译阶段（Gramine 容器）:"
echo "   ${BLUE}cd gramine${NC}"
echo "   ${BLUE}./build-in-gramine.sh${NC}"
echo ""
echo "3. 测试阶段:"
echo "   ${BLUE}./run-local.sh${NC}           # 快速集成测试"
echo "   ${BLUE}./run-dev.sh direct${NC}      # Gramine 模拟测试"
if [ "$SGX_AVAILABLE" = true ]; then
    echo "   ${BLUE}./run-dev.sh sgx${NC}         # SGX 真实测试"
fi
echo ""
echo "4. 部署阶段:"
echo "   ${BLUE}./build-docker.sh v1.0.0${NC} # 构建 Docker 镜像"
echo "   ${BLUE}docker-compose up -d${NC}     # 部署"
echo ""

# 总结
echo -e "${GREEN}=== 环境检查完成 ===${NC}"
if [ "$IN_GRAMINE" = true ]; then
    echo -e "${GREEN}✓ 环境已就绪，可以进行完整的编译和测试${NC}"
else
    echo -e "${YELLOW}⚠ 当前为宿主机环境，适合开发验证${NC}"
    echo -e "${YELLOW}  生产编译请使用 Gramine 容器${NC}"
fi
