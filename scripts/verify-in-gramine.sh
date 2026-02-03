#!/bin/bash
set -e

echo "============================================"
echo "MRENCLAVE Calculation Verification"
echo "Running in Gramine Docker Environment"
echo "============================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

# Start Gramine container
echo "Step 2: Starting Gramine Docker container..."
CONTAINER_NAME="gramine-verify-$$"
docker run -d --name "$CONTAINER_NAME" gramineproject/gramine:latest tail -f /dev/null
sleep 2
echo -e "${GREEN}✓ Container started: $CONTAINER_NAME${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Copy tool to container
echo "Step 3: Copying tool to container..."
docker cp calculate-mrenclave "$CONTAINER_NAME":/tmp/
echo -e "${GREEN}✓ Tool copied${NC}"
echo ""

# Create a simple test manifest in container
echo "Step 4: Creating test manifest in container..."
docker exec "$CONTAINER_NAME" bash -c 'cat > /tmp/test.manifest <<EOF
libos.entrypoint = "/bin/sh"

loader.log_level = "error"

sgx.enclave_size = "1G"
sgx.max_threads = 16
sgx.remote_attestation = "dcap"

sgx.trusted_files = [
  "file:{{ gramine.libos }}",
  "file:{{ gramine.runtimedir() }}/",
]
EOF'
echo -e "${GREEN}✓ Manifest created${NC}"
echo ""

# Generate manifest.sgx using Gramine
echo "Step 5: Generating manifest.sgx with gramine-sgx-sign..."
docker exec "$CONTAINER_NAME" bash -c 'cd /tmp && gramine-sgx-sign \
    --manifest test.manifest \
    --output test.manifest.sgx'

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Failed to generate manifest with gramine-sgx-sign${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Manifest signed by Gramine${NC}"
echo ""

# Extract Gramine's MRENCLAVE
echo "Step 6: Extracting Gramine's MRENCLAVE..."
GRAMINE_MR=$(docker exec "$CONTAINER_NAME" bash -c \
    'gramine-sgx-sigstruct-view /tmp/test.manifest.sgx 2>/dev/null | grep "mr_enclave" | awk "{print \$2}"')

if [ -z "$GRAMINE_MR" ]; then
    echo -e "${RED}✗ Failed to extract Gramine MRENCLAVE${NC}"
    exit 1
fi
echo -e "${YELLOW}Gramine MRENCLAVE: $GRAMINE_MR${NC}"
echo ""

# Run our tool to calculate MRENCLAVE
echo "Step 7: Running our calculate-mrenclave tool..."
OUR_OUTPUT=$(docker exec "$CONTAINER_NAME" /tmp/calculate-mrenclave /tmp/test.manifest.sgx 2>&1)
echo "$OUR_OUTPUT"
echo ""

# Extract our MRENCLAVE from output
OUR_MR=$(echo "$OUR_OUTPUT" | grep "Our calculated MRENCLAVE:" | awk '{print $4}')

if [ -z "$OUR_MR" ]; then
    echo -e "${RED}✗ Failed to calculate MRENCLAVE with our tool${NC}"
    exit 1
fi

# Compare MRENCLAVEs
echo "============================================"
echo "VERIFICATION RESULT"
echo "============================================"
echo ""
echo "Gramine MRENCLAVE: $GRAMINE_MR"
echo "Our MRENCLAVE:     $OUR_MR"
echo ""

if [ "$GRAMINE_MR" == "$OUR_MR" ]; then
    echo -e "${GREEN}✓✓✓ SUCCESS ✓✓✓${NC}"
    echo -e "${GREEN}MRENCLAVEs MATCH PERFECTLY!${NC}"
    echo -e "${GREEN}Our implementation is CORRECT and matches Gramine exactly.${NC}"
    exit 0
else
    echo -e "${RED}✗✗✗ FAILURE ✗✗✗${NC}"
    echo -e "${RED}MRENCLAVEs DO NOT MATCH!${NC}"
    echo -e "${RED}Our implementation needs to be debugged and fixed.${NC}"
    exit 1
fi
