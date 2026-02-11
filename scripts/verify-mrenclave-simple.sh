#!/bin/bash
set -e

echo "==================================="
echo "MRENCLAVE Verification Test"
echo "==================================="

# Build tool
echo "Building calculate-mrenclave..."
cd "$(dirname "$0")/.."
go build -o calculate-mrenclave ./cmd/calculate-mrenclave || {
    echo "Failed to build"
    exit 1
}

# Create a simple test to verify the tool works
echo ""
echo "Testing with existing manifest..."
if [ -f "gramine/test.manifest.sgx" ]; then
    ./calculate-mrenclave gramine/test.manifest.sgx
elif [ -f "gramine/geth.manifest.sgx" ]; then
    ./calculate-mrenclave gramine/geth.manifest.sgx  
else
    echo "No test manifest found. Tool is built and ready."
    echo "Run: ./calculate-mrenclave <path-to-manifest.sgx>"
fi

echo ""
echo "✓ Tool ready for verification in Gramine environment"
