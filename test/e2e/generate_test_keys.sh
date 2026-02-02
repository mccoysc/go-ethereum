#!/bin/bash

set -e

echo "=== Generating Test Signing Keys ==="

TEST_DIR="test/e2e"
KEY_DIR="$TEST_DIR/test-keys"

mkdir -p "$KEY_DIR"

# Generate RSA-3072 key pair for manifest signing (matching Gramine requirement)
if [ ! -f "$KEY_DIR/test-signing-key.pem" ]; then
    echo "Generating RSA-3072 private key..."
    openssl genrsa -3 -out "$KEY_DIR/test-signing-key.pem" 3072
    echo "✓ Private key generated"
fi

# Extract public key
if [ ! -f "$KEY_DIR/test-signing-key.pub" ]; then
    echo "Extracting public key..."
    openssl rsa -in "$KEY_DIR/test-signing-key.pem" -pubout -out "$KEY_DIR/test-signing-key.pub"
    echo "✓ Public key extracted"
fi

echo ""
echo "Keys generated successfully:"
echo "  Private key: $KEY_DIR/test-signing-key.pem"
echo "  Public key:  $KEY_DIR/test-signing-key.pub"
echo ""
echo "For testing, set:"
echo "  export GRAMINE_SIGSTRUCT_KEY_PATH=$PWD/$KEY_DIR/test-signing-key.pub"

