#!/bin/bash
set -e

echo "======================================"
echo "MRENCLAVE Verification - Single Docker Run"
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

# Run everything in single docker command
echo "Running verification in Gramine container..."
docker run --rm \
  -v /tmp/test.manifest.template:/tmp/test.manifest.template:ro \
  -v $(pwd)/calculate-mrenclave:/tmp/calculate-mrenclave:ro \
  gramineproject/gramine:latest \
  bash -c '
set -e
cd /tmp
echo "Step 1: Generating manifest..."
gramine-manifest test.manifest.template test.manifest
echo "Step 2: Signing manifest..."
gramine-sgx-sign --manifest test.manifest --output test.manifest.sgx
echo "Step 3: Extracting Gramine MRENCLAVE..."
GRAMINE_MR=$(gramine-sgx-sigstruct-view test.manifest.sgx 2>/dev/null | grep "mr_enclave" | head -1 | awk "{print \$2}")
echo "Gramine MRENCLAVE: $GRAMINE_MR"
echo ""
echo "Step 4: Running our calculate-mrenclave tool..."
/tmp/calculate-mrenclave test.manifest.sgx
echo ""
echo "======================================"
echo "Verification Complete"
echo "======================================"
'

echo "Done!"
