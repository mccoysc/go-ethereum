#!/bin/bash
set -e

echo "======================================"
echo "MRENCLAVE Verification with Log Capture"
echo "======================================"

# Build tool
echo "Building calculate-mrenclave..."
go build -o calculate-mrenclave ./cmd/calculate-mrenclave
echo "✓ Tool built"

# Create a simple test manifest
cat > /tmp/test.manifest.template << 'MANIFEST'
libos.entrypoint = "/bin/sh"

loader.log_level = "error"

sgx.enclave_size = "512M"
sgx.max_threads = 4

sgx.trusted_files = [
  "file:{{ gramine.libos }}",
  "file:{{ gramine.runtimedir() }}/",
]
MANIFEST

echo "Created test manifest template"

# Run in Docker with output capture
echo "Starting Gramine container..."
CONTAINER_ID=$(docker run -d gramineproject/gramine:latest sleep 60)
echo "Container ID: $CONTAINER_ID"

# Copy files
echo "Copying files to container..."
docker cp /tmp/test.manifest.template $CONTAINER_ID:/tmp/
docker cp calculate-mrenclave $CONTAINER_ID:/tmp/

# Generate manifest
echo "Generating manifest in container..."
docker exec $CONTAINER_ID bash -c "cd /tmp && gramine-manifest test.manifest.template test.manifest" 2>&1 | tee /tmp/gramine-manifest.log

echo "Signing manifest..."
docker exec $CONTAINER_ID bash -c "cd /tmp && gramine-sgx-sign --manifest test.manifest --output test.manifest.sgx" 2>&1 | tee /tmp/gramine-sign.log

# Extract Gramine's MRENCLAVE
echo "Extracting Gramine's MRENCLAVE..."
GRAMINE_MR=$(docker exec $CONTAINER_ID bash -c "gramine-sgx-sigstruct-view /tmp/test.manifest.sgx 2>/dev/null | grep 'mr_enclave' | awk '{print \$2}'")
echo "Gramine MRENCLAVE: $GRAMINE_MR"

# Run our tool
echo "Running our calculate-mrenclave tool..."
OUR_MR=$(docker exec $CONTAINER_ID /tmp/calculate-mrenclave /tmp/test.manifest.sgx 2>&1 | grep "MRENCLAVE:" | tail -1 | awk '{print $NF}')
echo "Our MRENCLAVE:     $OUR_MR"

# Compare
echo ""
echo "======================================"
echo "COMPARISON RESULT"
echo "======================================"
echo "Gramine: $GRAMINE_MR"
echo "Ours:    $OUR_MR"

if [ "$GRAMINE_MR" == "$OUR_MR" ]; then
    echo "✓✓✓ SUCCESS: MRENCLAVEs MATCH! ✓✓✓"
    RESULT=0
else
    echo "✗✗✗ FAILURE: MRENCLAVEs DO NOT MATCH ✗✗✗"
    RESULT=1
fi

# Cleanup
echo "Cleaning up container..."
docker stop $CONTAINER_ID > /dev/null
docker rm $CONTAINER_ID > /dev/null

exit $RESULT
