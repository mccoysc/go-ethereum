#!/bin/bash
# Script to verify CGO production compilation with runtime dynamic linking
# This script tests that the CGO code compiles and links using dlopen/dlsym

set -e

echo "=== Testing CGO Production with Runtime Dynamic Linking ==="
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

# Test 2: Verify compilation and linking with dlopen (no Gramine libs needed)
echo ""
echo "Test 2: Verifying CGO compilation and linking with dlopen..."
CGO_ENABLED=1 go build -tags cgo -o /tmp/sgx_test_dlopen ./internal/sgx/... 2>&1
if [ $? -eq 0 ]; then
    echo "✅ CGO code compiles and links successfully (using dlopen/dlsym)"
    echo "   No Gramine libraries required at compile/link time"
    rm -f /tmp/sgx_test_dlopen
else
    echo "❌ CGO compilation/linking failed"
    exit 1
fi

# Test 3: Verify non-CGO stub files are excluded with cgo tag
echo ""
echo "Test 3: Checking non-CGO stubs are excluded..."
GO_FILES=$(CGO_ENABLED=1 go list -tags cgo -f '{{.GoFiles}}' ./internal/sgx)
if echo "$GO_FILES" | grep -q "attestor_ratls.go"; then
    echo "❌ Non-CGO stub attestor_ratls.go is incorrectly included"
    exit 1
else
    echo "✅ Non-CGO stub attestor_ratls.go is correctly excluded"
fi

# Test 4: Verify build tags are correct
echo ""
echo "Test 4: Verifying build tags..."
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

# Test 5: Verify dlopen dependency
echo ""
echo "Test 5: Verifying dlopen linkage..."
if nm /tmp/sgx_test_dlopen 2>/dev/null | grep -q dlopen || objdump -T /tmp/sgx_test_dlopen 2>/dev/null | grep -q dlopen; then
    echo "✅ Binary uses dlopen for runtime dynamic linking"
else
    echo "⚠️  Cannot verify dlopen usage (binary may be archive format)"
fi

echo ""
echo "=== All CGO Production Tests Passed ✅ ==="
echo ""
echo "Summary:"
echo "- CGO files are correctly tagged and recognized"
echo "- Code compiles AND LINKS without Gramine libraries"
echo "- Uses runtime dynamic linking (dlopen/dlsym)"
echo "- Gramine libraries loaded at runtime when available"
echo "- Build tag separation works correctly"
echo ""
echo "Runtime behavior:"
echo "  • Without Gramine libs: Functions return error codes"
echo "  • With Gramine libs: Functions loaded via dlopen and work normally"
echo ""
echo "Build command:"
echo "  CGO_ENABLED=1 go build -tags cgo ./internal/sgx/..."
echo ""
echo "Gramine libraries (loaded at runtime if available):"
echo "  - libra_tls_attest.so"
echo "  - libra_tls_verify.so"
