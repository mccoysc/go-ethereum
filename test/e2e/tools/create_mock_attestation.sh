#!/bin/bash
set -e

# 模拟Gramine伪文件系统
# 创建 /dev/attestation/* 的模拟文件用于测试

MOCK_DEV_DIR="${1:-/tmp/mock-gramine-dev}"

echo "=== 创建Gramine伪文件系统模拟 ==="
echo "目标目录: ${MOCK_DEV_DIR}"

# 创建目录结构
mkdir -p "${MOCK_DEV_DIR}/attestation"

echo "[1/4] 创建 /dev/attestation/type ..."
# type文件包含attestation类型
echo "dcap" > "${MOCK_DEV_DIR}/attestation/type"
echo "  ✓ 已创建: ${MOCK_DEV_DIR}/attestation/type"

echo "[2/4] 创建 /dev/attestation/user_report_data (可写入64字节) ..."
# 这个文件用于接收应用写入的report data
# 创建一个空文件，应用会写入64字节
touch "${MOCK_DEV_DIR}/attestation/user_report_data"
chmod 666 "${MOCK_DEV_DIR}/attestation/user_report_data"
echo "  ✓ 已创建: ${MOCK_DEV_DIR}/attestation/user_report_data"

echo "[3/4] 创建 /dev/attestation/quote (包含模拟SGX quote) ..."
# 创建一个模拟的SGX quote
# 真实的quote大小通常是几KB，这里创建一个简化版本
python3 - << 'PYTHON_EOF'
import struct
import hashlib
import os

# SGX Quote 结构（简化版）
# 实际quote更复杂，这里只包含关键字段

quote = bytearray()

# Quote Header (48 bytes)
quote += struct.pack('<H', 3)  # Version (3 for DCAP)
quote += struct.pack('<H', 0)  # Attestation Key Type (0 = ECDSA-256 with P-256)
quote += struct.pack('<I', 0)  # TEE Type (0 = SGX)
quote += b'\x00' * 2          # Reserved
quote += b'\x00' * 2          # Reserved
quote += b'\x00' * 16         # QE Vendor ID
quote += b'\x00' * 20         # User Data (first 20 bytes)

# ISV Enclave Report (384 bytes)
report = bytearray(384)

# CPU SVN (16 bytes)
report[0:16] = b'\x04\x04\x02\x0f\x80\x70\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00'

# MISC SELECT (4 bytes)
struct.pack_into('<I', report, 16, 0)

# Reserved (28 bytes)
report[20:48] = b'\x00' * 28

# Attributes (16 bytes) - SGX flags and XFRM
struct.pack_into('<Q', report, 48, 0x0000000000000007)  # FLAGS: INIT | MODE64BIT | PROVISIONKEY
struct.pack_into('<Q', report, 56, 0x000000000000001f)  # XFRM

# MRENCLAVE (32 bytes) - 使用一个测试值
# 这应该与manifest的哈希相同
test_mrenclave = hashlib.sha256(b'test-enclave-measurement').digest()
report[64:96] = test_mrenclave

# Reserved (32 bytes)
report[96:128] = b'\x00' * 32

# MRSIGNER (32 bytes) - 使用一个测试值
test_mrsigner = hashlib.sha256(b'test-signer-key').digest()
report[128:160] = test_mrsigner

# Reserved (96 bytes)
report[160:256] = b'\x00' * 96

# ISV PROD ID (2 bytes)
struct.pack_into('<H', report, 256, 0)

# ISV SVN (2 bytes)
struct.pack_into('<H', report, 258, 0)

# Reserved (60 bytes)
report[260:320] = b'\x00' * 60

# Report Data (64 bytes) - 从user_report_data文件读取
user_data_file = os.environ.get('MOCK_DEV_DIR', '/tmp/mock-gramine-dev') + '/attestation/user_report_data'
try:
    with open(user_data_file, 'rb') as f:
        user_data = f.read(64)
        if len(user_data) < 64:
            user_data += b'\x00' * (64 - len(user_data))
        report[320:384] = user_data[:64]
except:
    report[320:384] = b'\x00' * 64

# 组装quote
quote += report

# Quote Signature (variable length, simplified)
# 真实的quote还包含签名和证书链，这里简化
quote += b'\x00' * 64  # Simplified signature

# 写入quote文件
quote_file = os.environ.get('MOCK_DEV_DIR', '/tmp/mock-gramine-dev') + '/attestation/quote'
with open(quote_file, 'wb') as f:
    f.write(quote)

print(f"  ✓ 已创建: {quote_file}")
print(f"  Quote大小: {len(quote)} bytes")
print(f"  MRENCLAVE: {test_mrenclave.hex()}")
print(f"  MRSIGNER: {test_mrsigner.hex()}")
PYTHON_EOF

echo "[4/4] 设置文件权限 ..."
# 先确保文件可写，以便可以重复运行脚本
chmod 644 "${MOCK_DEV_DIR}/attestation/type" 2>/dev/null || true
chmod 666 "${MOCK_DEV_DIR}/attestation/user_report_data" 2>/dev/null || true
chmod 644 "${MOCK_DEV_DIR}/attestation/quote" 2>/dev/null || true
# 再设置为最终权限
chmod 644 "${MOCK_DEV_DIR}/attestation/type"
chmod 666 "${MOCK_DEV_DIR}/attestation/user_report_data"
chmod 444 "${MOCK_DEV_DIR}/attestation/quote"
echo "  ✓ 权限已设置"

echo ""
echo "=== 模拟文件系统已创建 ==="
echo "目录: ${MOCK_DEV_DIR}"
echo ""
echo "文件列表:"
ls -lh "${MOCK_DEV_DIR}/attestation/"
echo ""
echo "要在测试中使用，需要符号链接或bind mount:"
echo "sudo mkdir -p /dev/attestation"
echo "sudo mount --bind ${MOCK_DEV_DIR}/attestation /dev/attestation"
echo ""
echo "或者修改代码使用环境变量指定路径:"
echo "export GRAMINE_ATTESTATION_PATH=${MOCK_DEV_DIR}/attestation"
echo ""
