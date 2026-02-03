#!/bin/bash
set -e

echo "======================================"
echo "MRENCLAVE Verification - Final Test"
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

echo "✓ Created test manifest template"

# Run everything in one docker exec to keep container alive
echo "Starting verification in Gramine container..."
docker run --rm -v /tmp:/host -v $(pwd)/calculate-mrenclave:/tool gramineproject/gramine:latest bash -c "
set -e
cd /tmp
cp /host/test.manifest.template .
echo 'Generating manifest...'
gramine-manifest test.manifest.template test.manifest
echo 'Signing manifest...'
gramine-sgx-sign --manifest test.manifest --output test.manifest.sgx
echo 'Extracting Gramine MRENCLAVE...'
GRAMINE_MR=\$(gramine-sgx-sigstruct-view test.manifest.sgx 2>/dev/null | grep 'mr_enclave' | head -1 | awk '{print \$2}')
echo \"Gramine MRENCLAVE: \$GRAMINE_MR\"
echo 'Running our calculator...'
/tool test.manifest.sgx
" 2>&1 | tee /tmp/verification.log

echo ""
echo "======================================"
echo "Verification complete - check output above"
echo "======================================"
