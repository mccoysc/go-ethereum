#!/bin/bash
set -e

# 模拟Gramine伪文件系统
# 创建 /dev/attestation/* 的模拟文件用于测试

MOCK_DEV_DIR="${1:-/tmp/mock-gramine-dev}"
DATA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/data"

echo "=== 创建Gramine伪文件系统模拟 ==="
echo "目标目录: ${MOCK_DEV_DIR}"

# 创建目录结构
mkdir -p "${MOCK_DEV_DIR}/attestation"

# [1/5] 创建 /dev/attestation/type
echo "[1/5] 创建 /dev/attestation/type ..."
echo "dcap" > "${MOCK_DEV_DIR}/attestation/type"
echo "  ✓ 已创建: ${MOCK_DEV_DIR}/attestation/type"

# [2/5] 创建 /dev/attestation/my_target_info (包含当前enclave的MRENCLAVE)
echo "[2/5] 创建 /dev/attestation/my_target_info (包含当前enclave的MRENCLAVE) ..."

# 从manifest计算MRENCLAVE（与create_test_manifest.sh保持一致）
MANIFEST_FILE="${DATA_DIR}/geth.manifest"
if [ -f "$MANIFEST_FILE" ]; then
    MRENCLAVE=$(sha256sum "$MANIFEST_FILE" | awk '{print $1}')
    echo "  使用manifest哈希作为MRENCLAVE: $MRENCLAVE"
else
    # 使用默认测试值
    MRENCLAVE="4b558ddc7e75a2976005fe08371067eba9692c022740ca211a3ee3c059973706"
    echo "  使用默认测试MRENCLAVE: $MRENCLAVE"
fi

# 创建TARGETINFO结构（512字节）
# MRENCLAVE在offset 0，32字节
python3 << PYTHON_SCRIPT
mrenclave_hex = "${MRENCLAVE}"
mrenclave_bytes = bytes.fromhex(mrenclave_hex)

target_info = bytearray(512)
target_info[0:32] = mrenclave_bytes  # MRENCLAVE at offset 0

# 写入文件
output_file = "${MOCK_DEV_DIR}/attestation/my_target_info"
with open(output_file, 'wb') as f:
    f.write(target_info)

print(f"  ✓ 已创建: {output_file}")
print(f"  TARGETINFO大小: {len(target_info)} bytes")
print(f"  MRENCLAVE (offset 0): {mrenclave_hex}")
PYTHON_SCRIPT

# [3/5] 创建 /dev/attestation/user_report_data (可写)
echo "[3/5] 创建 /dev/attestation/user_report_data (可写入64字节) ..."
touch "${MOCK_DEV_DIR}/attestation/user_report_data"
chmod 666 "${MOCK_DEV_DIR}/attestation/user_report_data"
echo "  ✓ 已创建: ${MOCK_DEV_DIR}/attestation/user_report_data"

# [4/5] 创建 /dev/attestation/quote (包含模拟SGX quote)
echo "[4/5] 创建 /dev/attestation/quote (包含模拟SGX quote) ..."
# 先删除可能存在的只读文件
rm -f "$MOCK_DEV_DIR/attestation/quote" 2>/dev/null || true

python3 << 'PYTHON_EOF'
import struct
import hashlib
import os

output_dir = os.environ.get('MOCK_DEV_DIR', '/tmp/mock-gramine-dev/attestation')
quote_path = os.path.join(output_dir, 'attestation/quote')

# 生成模拟的SGX quote
quote = bytearray()

# Quote Header (48 bytes)
quote += struct.pack('<H', 3)  # Version
quote += struct.pack('<H', 0)  # Attestation Key Type
quote += struct.pack('<I', 0)  # TEE Type
quote += b'\x00' * 38

# ISV Enclave Report (384 bytes)
report = bytearray(384)

# CPU SVN (16 bytes)
report[0:16] = b'\x00' * 16

# Misc Select (4 bytes)
struct.pack_into('<I', report, 16, 0)

# Reserved (28 bytes)
report[20:48] = b'\x00' * 28

# Attributes (16 bytes)
struct.pack_into('<Q', report, 48, 0x0000000000000007)
struct.pack_into('<Q', report, 56, 0x000000000000001f)

# MRENCLAVE (32 bytes) - 从my_target_info读取相同的值
mrenclave_file = os.path.join(output_dir, 'attestation/my_target_info')
if os.path.exists(mrenclave_file):
    with open(mrenclave_file, 'rb') as f:
        target_info = f.read()
        test_mrenclave = target_info[0:32]
else:
    test_mrenclave = hashlib.sha256(b'test-enclave-measurement').digest()
report[64:96] = test_mrenclave

# Reserved (32 bytes)
report[96:128] = b'\x00' * 32

# MRSIGNER (32 bytes)
test_mrsigner = hashlib.sha256(b'test-signer-key').digest()
report[128:160] = test_mrsigner

# Reserved (96 bytes) + ISV PROD ID + ISV SVN + Reserved
report[160:320] = b'\x00' * 160

# Report Data (64 bytes)
user_data_file = os.path.join(output_dir, 'attestation/user_report_data')
if os.path.exists(user_data_file):
    with open(user_data_file, 'rb') as f:
        user_data = f.read()
        if len(user_data) >= 64:
            report[320:384] = user_data[0:64]
        else:
            report[320:320+len(user_data)] = user_data
            report[320+len(user_data):384] = b'\x00' * (64 - len(user_data))
else:
    report[320:384] = b'\x00' * 64

quote += report

# Quote signature (simplified)
quote += b'\x00' * 64

# 写入quote文件
with open(quote_path, 'wb') as f:
    f.write(quote)

print(f"  ✓ 已创建: {quote_path}")
print(f"  Quote大小: {len(quote)} bytes")
print(f"  MRENCLAVE: {test_mrenclave.hex()}")
print(f"  MRSIGNER: {test_mrsigner.hex()}")
PYTHON_EOF

# [5/5] 设置文件权限
echo "[5/5] 设置文件权限 ..."
chmod 644 "${MOCK_DEV_DIR}/attestation/type" 2>/dev/null || true
chmod 444 "${MOCK_DEV_DIR}/attestation/my_target_info"  # 只读
chmod 666 "${MOCK_DEV_DIR}/attestation/user_report_data"  # 可写
chmod 444 "${MOCK_DEV_DIR}/attestation/quote"  # 只读
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
echo "MRENCLAVE已写入my_target_info，manifest验证将使用此值"
