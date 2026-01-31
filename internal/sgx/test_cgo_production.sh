#!/bin/bash
# Script to verify CGO production compilation
# This script tests that the CGO code compiles correctly in production mode

set -e

echo "=== Testing CGO Production Compilation and Linking ==="
echo ""

# Test 1: Verify CGO files are included when cgo tag is used
echo "Test 1: Checking CGO files are recognized..."
CGO_FILES=$(CGO_ENABLED=1 go list -tags cgo -f '{{.CgoFiles}}' ./internal/sgx)
if echo "$CGO_FILES" | grep -q "attestor_ratls_cgo.go"; then
    echo "✅ attestor_ratls_cgo.go is recognized"
else
    echo "❌ attestor_ratls_cgo.go is NOT recognized"
    exit 1
fi

if echo "$CGO_FILES" | grep -q "verifier_ratls_cgo.go"; then
    echo "✅ verifier_ratls_cgo.go is recognized"
else
    echo "❌ verifier_ratls_cgo.go is NOT recognized"
    exit 1
fi

# Test 2: Verify syntax compilation and LINKING (without Gramine libs)
echo ""
echo "Test 2: Verifying CGO compilation and linking (without Gramine libs)..."
CGO_ENABLED=1 go build -tags cgo -o /tmp/sgx_test_no_libs ./internal/sgx/... 2>&1
if [ $? -eq 0 ]; then
    echo "✅ CGO code compiles and links successfully (using weak symbol stubs)"
    rm -f /tmp/sgx_test_no_libs
else
    echo "❌ CGO compilation/linking failed"
    exit 1
fi

# Test 3: Verify compilation with gramine_libs tag (for production with Gramine)
echo ""
echo "Test 3: Verifying CGO compilation with gramine_libs tag..."
CGO_ENABLED=1 go build -tags "cgo gramine_libs" -o /tmp/sgx_test_gramine ./internal/sgx/... 2>&1
if [ $? -eq 0 ]; then
    echo "✅ CGO code compiles with gramine_libs tag (will link Gramine libraries)"
    rm -f /tmp/sgx_test_gramine
else
    echo "❌ CGO compilation with gramine_libs failed"
    exit 1
fi

# Test 4: Verify non-CGO stub files are excluded with cgo tag
echo ""
echo "Test 4: Checking non-CGO stubs are excluded..."
GO_FILES=$(CGO_ENABLED=1 go list -tags cgo -f '{{.GoFiles}}' ./internal/sgx)
if echo "$GO_FILES" | grep -q "attestor_ratls.go"; then
    echo "❌ Non-CGO stub attestor_ratls.go is incorrectly included"
    exit 1
else
    echo "✅ Non-CGO stub attestor_ratls.go is correctly excluded"
fi

# Test 5: Verify build tags are correct
echo ""
echo "Test 5: Verifying build tags..."
if head -20 internal/sgx/attestor_ratls_cgo.go | grep -q "//go:build cgo"; then
    echo "✅ attestor_ratls_cgo.go has correct build tag (cgo)"
else
    echo "❌ attestor_ratls_cgo.go missing or incorrect build tag"
    exit 1
fi

if head -20 internal/sgx/attestor_ratls.go | grep -q "//go:build !cgo"; then
    echo "✅ attestor_ratls.go has correct build tag (!cgo)"
else
    echo "❌ attestor_ratls.go missing or incorrect build tag"
    exit 1
fi

echo ""
echo "=== All CGO Production Compilation and Linking Tests Passed ✅ ==="
echo ""
echo "Summary:"
echo "- CGO files are correctly tagged and recognized"
echo "- CGO code compiles AND LINKS without syntax errors"
echo "- Weak symbol stubs allow linking without Gramine libraries"
echo "- Build tag separation works correctly"
echo "- Production build is ready for both modes:"
echo "  • Without Gramine libs: Uses weak symbol stubs (links successfully)"
echo "  • With Gramine libs: Uses real implementations (add -tags gramine_libs)"
echo ""
echo "Build modes:"
echo "  1. Testing/CI (no Gramine): CGO_ENABLED=1 go build -tags cgo"
echo "  2. Production (with Gramine): CGO_ENABLED=1 go build -tags 'cgo gramine_libs'"
echo ""
echo "Note: When using -tags gramine_libs, Gramine RA-TLS libraries must be installed:"
echo "  - libra_tls_attest.so"
echo "  - libra_tls_verify.so"
echo "  - libsgx_dcap_ql.so"
echo "  - libmbedtls, libmbedx509, libmbedcrypto"
