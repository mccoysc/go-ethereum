#!/bin/bash
set -e

echo "============================================"
echo "MRENCLAVE Calculation Verification"
echo "Running in Gramine Docker Environment"
echo "============================================"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Build our tool
echo "Step 1: Building calculate-mrenclave tool..."
cd "$(dirname "$0")/.."
go build -o calculate-mrenclave ./cmd/calculate-mrenclave
if [ ! -f "calculate-mrenclave" ]; then
    echo -e "${RED}✗ Failed to build calculate-mrenclave${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Tool built successfully${NC}"
echo ""

# Run all verification steps in a single docker run command
echo "Step 2: Running verification in Gramine container..."
docker run --rm \
  -v "$(pwd)/calculate-mrenclave:/tmp/calculate-mrenclave" \
  gramineproject/gramine:latest \
  bash -c '
set -e

echo "Creating test manifest..."
cat > /tmp/test.manifest <<EOF
libos.entrypoint = "/bin/sh"
loader.log_level = "error"
sgx.enclave_size = "1G"
sgx.max_threads = 16
sgx.remote_attestation = "dcap"
sgx.trusted_files = [
  "file:{{ gramine.libos }}",
  "file:{{ gramine.runtimedir() }}/",
]
EOF

echo "Generating RSA key..."
mkdir -p ~/.config/gramine
gramine-sgx-gen-private-key

echo ""
echo "Generating manifest.sgx with gramine-sgx-sign..."
cd /tmp
gramine-sgx-sign --manifest test.manifest --output test.manifest.sgx

echo ""
echo "Extracting Gramine MRENCLAVE..."
GRAMINE_MR=$(gramine-sgx-sigstruct-view test.manifest.sgx 2>/dev/null | grep "mr_enclave" | awk "{print \$2}")
echo "Gramine MRENCLAVE: $GRAMINE_MR"

echo ""
echo "Running our calculate-mrenclave tool..."
chmod +x /tmp/calculate-mrenclave
OUR_OUTPUT=$(/tmp/calculate-mrenclave /tmp/test.manifest.sgx 2>&1)
echo "$OUR_OUTPUT"

# Extract our MRENCLAVE from output
OUR_MR=$(echo "$OUR_OUTPUT" | grep "Our calculated MRENCLAVE" | awk "{print \$NF}")

echo ""
echo "============================================"
echo "COMPARISON RESULTS:"
echo "============================================"
echo "Gramine MRENCLAVE: $GRAMINE_MR"
echo "Our MRENCLAVE:     $OUR_MR"
echo ""

if [ "$GRAMINE_MR" == "$OUR_MR" ]; then
    echo "✓ SUCCESS: MRENCLAVEs MATCH!"
    echo "Our implementation is CORRECT and matches Gramine exactly."
    exit 0
else
    echo "✗ FAILURE: MRENCLAVEs DO NOT MATCH!"
    echo "Our implementation needs to be fixed."
    exit 1
fi
'

echo ""
echo -e "${GREEN}Verification complete!${NC}"
