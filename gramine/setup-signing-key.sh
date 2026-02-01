#!/bin/bash
# setup-signing-key.sh
# 生成 Gramine manifest 签名密钥

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

KEY_FILE="${SCRIPT_DIR}/enclave-key.pem"

echo -e "${GREEN}=== Gramine 签名密钥设置 ===${NC}"

if [ -f "${KEY_FILE}" ]; then
    echo -e "${YELLOW}签名密钥已存在: ${KEY_FILE}${NC}"
    
    # 显示密钥信息
    echo ""
    echo "密钥信息:"
    openssl rsa -in "${KEY_FILE}" -text -noout | head -5
    
    read -p "是否要重新生成密钥? 这会改变 MRENCLAVE! (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "保持现有密钥"
        exit 0
    fi
    
    # 备份旧密钥
    BACKUP="${KEY_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
    mv "${KEY_FILE}" "${BACKUP}"
    echo -e "${YELLOW}旧密钥已备份到: ${BACKUP}${NC}"
fi

echo "生成新的 RSA 3072 位签名密钥..."
openssl genrsa -3 -out "${KEY_FILE}" 3072

chmod 600 "${KEY_FILE}"

echo -e "${GREEN}✓ 签名密钥已生成: ${KEY_FILE}${NC}"
echo ""
echo "密钥信息:"
openssl rsa -in "${KEY_FILE}" -text -noout | head -5

echo ""
echo -e "${YELLOW}重要提示:${NC}"
echo "  1. 此密钥影响 MRSIGNER 值"
echo "  2. 开发模式使用 MRSIGNER sealing，重新生成密钥后会无法访问旧数据"
echo "  3. 生产环境应妥善保管此密钥"
echo "  4. 不要将此密钥提交到 Git 仓库"
