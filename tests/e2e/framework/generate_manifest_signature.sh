#!/bin/bash
# Generate real RSA key pair and sign manifest for testing

set -e

MANIFEST_DIR="${1:-/tmp/xchain-test-manifest}"
mkdir -p "$MANIFEST_DIR"

echo "Generating RSA-3072 key pair for manifest signing..."

# Generate RSA-3072 private key (Gramine standard)
openssl genrsa -out "$MANIFEST_DIR/enclave-key.pem" 3072 2>/dev/null

# Extract public key in PEM format
openssl rsa -in "$MANIFEST_DIR/enclave-key.pem" -pubout -out "$MANIFEST_DIR/enclave-key.pub" 2>/dev/null

echo "✓ RSA key pair generated"
echo "  Private key: $MANIFEST_DIR/enclave-key.pem"
echo "  Public key: $MANIFEST_DIR/enclave-key.pub"

# Create manifest file
cat > "$MANIFEST_DIR/geth.manifest.sgx" << 'MANIFEST_EOF'
# Gramine Manifest for Testing
libos.entrypoint = "/app/geth"

# Environment variables - Contract addresses (security critical)
loader.env.XCHAIN_GOVERNANCE_CONTRACT = "0xd9145CCE52D386f254917e481eB44e9943F39138"
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

# SGX configuration
sgx.enclave_size = "2G"
sgx.max_threads = 32
sgx.remote_attestation = "dcap"
MANIFEST_EOF

echo "✓ Manifest file created: $MANIFEST_DIR/geth.manifest.sgx"

# Sign the manifest with private key using pkeyutl (PKCS#1 v1.5 padding)
openssl dgst -sha256 -sign "$MANIFEST_DIR/enclave-key.pem" \
    -out "$MANIFEST_DIR/geth.manifest.sgx.sig" \
    "$MANIFEST_DIR/geth.manifest.sgx"

echo "✓ Manifest signed: $MANIFEST_DIR/geth.manifest.sgx.sig"

# Verify signature
if openssl dgst -sha256 -verify "$MANIFEST_DIR/enclave-key.pub" \
    -signature "$MANIFEST_DIR/geth.manifest.sgx.sig" \
    "$MANIFEST_DIR/geth.manifest.sgx" >/dev/null 2>&1; then
    echo "✓ Signature verification successful!"
else
    echo "✗ Signature verification failed!"
    exit 1
fi

echo ""
echo "Manifest signing complete. Files created:"
echo "  - Manifest: $MANIFEST_DIR/geth.manifest.sgx"
echo "  - Signature: $MANIFEST_DIR/geth.manifest.sgx.sig"
echo "  - Public key: $MANIFEST_DIR/enclave-key.pub"
echo "  - Private key: $MANIFEST_DIR/enclave-key.pem (keep secure!)"
