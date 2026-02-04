#!/bin/bash
set -e

echo "======================================"
echo "MRENCLAVE Verification with Docker Exec"
echo "======================================"

# Build tool
echo "Building calculate-mrenclave..."
go build -o calculate-mrenclave ./cmd/calculate-mrenclave
echo "✓ Tool built"

# Create test manifest
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

echo "✓ Created test manifest"

# Start container with tail -f to keep it running
echo "Starting Gramine container..."
CONTAINER_ID=$(docker run -d gramineproject/gramine:latest tail -f /dev/null)
echo "Container ID: $CONTAINER_ID"

# Give container time to start
sleep 2

# Copy files
echo "Copying files to container..."
docker cp /tmp/test.manifest.template $CONTAINER_ID:/tmp/
docker cp calculate-mrenclave $CONTAINER_ID:/tmp/

# Generate manifest
echo "Generating manifest..."
docker exec $CONTAINER_ID bash -c "cd /tmp && gramine-manifest test.manifest.template test.manifest"

# Sign manifest
echo "Signing manifest..."
docker exec $CONTAINER_ID bash -c "cd /tmp && gramine-sgx-sign --manifest test.manifest --output test.manifest.sgx"

# Extract Gramine's MRENCLAVE
echo "Extracting Gramine's MRENCLAVE..."
GRAMINE_MR=$(docker exec $CONTAINER_ID bash -c "gramine-sgx-sigstruct-view /tmp/test.manifest.sgx 2>/dev/null | grep 'mr_enclave' | head -1 | awk '{print \$2}'")
echo "Gramine MRENCLAVE: $GRAMINE_MR"

# Run our tool
echo "Running our calculate-mrenclave tool..."
docker exec $CONTAINER_ID /tmp/calculate-mrenclave /tmp/test.manifest.sgx

# Cleanup
echo "Stopping container..."
docker stop $CONTAINER_ID
docker rm $CONTAINER_ID

echo "Done!"
