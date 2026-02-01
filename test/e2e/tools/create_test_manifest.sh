#!/bin/bash
set -e

# 创建真实的可验证manifest文件

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${TEST_DIR}/data"

mkdir -p "${OUTPUT_DIR}"

echo "=== 创建测试Manifest文件 ==="

# 1. 生成RSA-3072密钥对
echo "[1/4] 生成RSA-3072签名密钥..."
SIGNING_KEY="${OUTPUT_DIR}/test-signing-key.pem"
PUBLIC_KEY="${OUTPUT_DIR}/test-signing-key.pub"

if [ ! -f "${SIGNING_KEY}" ]; then
    openssl genrsa -3 -out "${SIGNING_KEY}" 3072
    openssl rsa -in "${SIGNING_KEY}" -pubout -out "${PUBLIC_KEY}"
    echo "  ✓ 密钥已生成: ${SIGNING_KEY}"
else
    echo "  ✓ 使用现有密钥: ${SIGNING_KEY}"
fi

# 2. 创建manifest文件
echo "[2/4] 创建manifest文件..."
MANIFEST_FILE="${OUTPUT_DIR}/geth.manifest"

cat > "${MANIFEST_FILE}" << 'EOF'
# Geth manifest for Gramine SGX

loader.entrypoint = "file:{{ gramine.libos }}"
libos.entrypoint = "/usr/local/bin/geth"

loader.log_level = "error"

loader.env.LD_LIBRARY_PATH = "/lib:/usr/lib:/usr/local/lib"
loader.env.PATH = "/usr/local/bin:/usr/bin:/bin"

# SGX Configuration
sgx.debug = false
sgx.edmm_enable = false
sgx.enclave_size = "4G"
sgx.max_threads = 32
sgx.remote_attestation = "dcap"

# X Chain Contract Addresses (read by geth at startup)
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0x0000000000000000000000000000000000001001"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0x0000000000000000000000000000000000001002"
loader.env.XCHAIN_INCENTIVE_CONTRACT = "0x0000000000000000000000000000000000001003"

# Gramine version
loader.env.GRAMINE_VERSION = "1.6"

# File system
fs.mounts = [
  { path = "/lib", uri = "file:{{ gramine.runtimedir() }}" },
  { path = "/usr/lib", uri = "file:/usr/lib" },
  { path = "/usr/local/bin/geth", uri = "file:/usr/local/bin/geth" },
  { path = "/etc", uri = "file:/etc" },
  { path = "/tmp", type = "tmpfs" },
]

# Allowed files
sgx.allowed_files = [
  "file:/etc/nsswitch.conf",
  "file:/etc/ethers",
  "file:/etc/hosts",
  "file:/etc/group",
  "file:/etc/passwd",
]

sgx.trusted_files = [
  "file:{{ gramine.libos }}",
  "file:{{ gramine.runtimedir() }}/",
  "file:/usr/lib/",
  "file:/usr/local/bin/geth",
]
EOF

echo "  ✓ Manifest已创建: ${MANIFEST_FILE}"

# 3. 计算manifest的SHA256哈希
echo "[3/4] 计算manifest哈希..."
MANIFEST_HASH=$(sha256sum "${MANIFEST_FILE}" | awk '{print $1}')
echo "  Manifest SHA256: ${MANIFEST_HASH}"

# 4. 使用RSA-3072签名manifest
echo "[4/4] 签名manifest文件..."
SIGNATURE_FILE="${OUTPUT_DIR}/geth.manifest.sig"

# 使用RSA签名（PKCS#1 v1.5）
openssl dgst -sha256 -sign "${SIGNING_KEY}" -out "${SIGNATURE_FILE}.raw" "${MANIFEST_FILE}"

# 创建模拟的SIGSTRUCT格式文件（1808字节）
python3 - << PYTHON_EOF
import struct

# 读取RSA签名
with open('${SIGNATURE_FILE}.raw', 'rb') as f:
    rsa_signature = f.read()

# 确保签名是384字节（RSA-3072 = 3072 bits = 384 bytes）
if len(rsa_signature) != 384:
    if len(rsa_signature) < 384:
        rsa_signature = rsa_signature + b'\x00' * (384 - len(rsa_signature))
    else:
        rsa_signature = rsa_signature[:384]

# 创建SIGSTRUCT结构（1808字节）
sigstruct = bytearray(1808)

# Header (offset 0, 16 bytes)
sigstruct[0:16] = b'\x06\x00\x00\x00\xE1\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00'

# Vendor (offset 16, 4 bytes) - Intel = 0x8086
struct.pack_into('<I', sigstruct, 16, 0x8086)

# Date (offset 20, 4 bytes) - 20260201
struct.pack_into('<I', sigstruct, 20, 20260201)

# SW Defined (offset 24, 4 bytes)
struct.pack_into('<I', sigstruct, 24, 0)

# Signature (offset 128, 384 bytes)
sigstruct[128:512] = rsa_signature

# Mock MRENCLAVE (offset 960, 32 bytes) - use manifest hash
manifest_hash = bytes.fromhex('${MANIFEST_HASH}')
sigstruct[960:992] = manifest_hash

# Write SIGSTRUCT file
with open('${SIGNATURE_FILE}', 'wb') as f:
    f.write(sigstruct)

print(f"✓ SIGSTRUCT文件已创建: ${SIGNATURE_FILE}")
print(f"  大小: {len(sigstruct)} bytes")
print(f"  MRENCLAVE: {manifest_hash.hex()}")
PYTHON_EOF

rm -f "${SIGNATURE_FILE}.raw"

echo ""
echo "=== 测试文件已创建 ==="
echo "Manifest: ${MANIFEST_FILE}"
echo "Signature: ${SIGNATURE_FILE}"  
echo "Public Key: ${PUBLIC_KEY}"
echo "Private Key: ${SIGNING_KEY}"
echo ""
echo "设置环境变量："
echo "export GRAMINE_MANIFEST_PATH=${MANIFEST_FILE}"
echo "export GRAMINE_SIGSTRUCT_KEY_PATH=${PUBLIC_KEY}"
echo "export RA_TLS_MRENCLAVE=${MANIFEST_HASH}"
echo "export GRAMINE_VERSION=1.6"
echo ""
