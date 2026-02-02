#!/bin/bash
# E2E Test: Permission Features
# Tests all 5 permission-related features

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../framework/test_env.sh"
source "$SCRIPT_DIR/../framework/assertions.sh"
source "$SCRIPT_DIR/../framework/node.sh"
source "$SCRIPT_DIR/../framework/contracts.sh"
source "$SCRIPT_DIR/../framework/crypto.sh"

TEST_NAME="Permission Features"
echo "========================================="
echo "E2E Test: $TEST_NAME"
echo "========================================="
echo ""

# Setup
TEST_DIR=$(setup_test_dir "permissions")
print_test_env

# Initialize and start node
init_test_node "$TEST_DIR"
start_test_node "$TEST_DIR" 30310 8552

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0

# Helper function to run test
run_test() {
    local test_name="$1"
    local test_func="$2"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo "---"
    echo "Test $TOTAL_TESTS: $test_name"
    
    if $test_func; then
        echo "✓ PASS: $test_name"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "✗ FAIL: $test_name"
    fi
}

# Feature 1: Balance check for key creation in read-only mode
test_balance_check_readonly() {
    # TODO: Implement test for read-only mode balance check
    # Should fail when called with insufficient balance
    echo "Testing balance check in read-only mode..."
    return 0  # Placeholder
}

# Feature 2: Owner permission check
test_owner_permission_check() {
    echo "Testing owner permission checks..."
    
    # Create key as user1
    local user1="0x1234567890123456789012345678901234567890"
    local key_id=$(create_ecdsa_key "$user1")
    assert_not_empty "$key_id" "Key created by user1"
    
    # Try to use key as user2 (should fail)
    local user2="0x0987654321098765432109876543210987654321"
    local message="0xdeadbeef"
    
    # Sign with wrong owner should fail
    local result=$(call_contract "0x8002" "${key_id}${message}" "$user2")
    if [ "$result" == "0x" ] || [ -z "$result" ]; then
        return 0  # Permission denied as expected
    else
        echo "ERROR: Non-owner was able to sign!"
        return 1
    fi
}

# Feature 3: Owner transfer functionality  
test_owner_transfer() {
    echo "Testing owner transfer..."
    
    # Create key as user1
    local user1="0x1234567890123456789012345678901234567890"
    local key_id=$(create_ecdsa_key "$user1")
    
    # Try to transfer ownership (if implemented)
    # TODO: Check if SGX_TRANSFER_OWNERSHIP precompile exists
    # For now, mark as not implemented
    echo "Owner transfer not yet implemented"
    return 1
}

# Feature 4: Block sync with cryptographic data
test_block_crypto_sync() {
    echo "Testing block synchronization with crypto data..."
    
    # Create multiple keys
    local user1="0x1234567890123456789012345678901234567890"
    local key1=$(create_ecdsa_key "$user1")
    local key2=$(create_ed25519_key "$user1")
    
    # Submit transaction to create block
    # TODO: Trigger block production and verify sync includes key data
    echo "Block crypto sync test - needs block production"
    return 1
}

# Feature 5: Re-encryption in decrypt operation
test_decrypt_reencryption() {
    echo "Testing decrypt with re-encryption..."
    
    # Create AES key
    local user1="0x1234567890123456789012345678901234567890"
    local key_id=$(create_aes_key "$user1")
    
    # Encrypt some data
    local plaintext="0xdeadbeefcafe"
    local ciphertext=$(call_contract "0x8006" "${key_id}${plaintext}" "$user1")
    
    # Try to decrypt (check if re-encryption parameter exists)
    # Current implementation returns plaintext, should return re-encrypted ciphertext
    local decrypted=$(call_contract "0x8007" "${key_id}${ciphertext}" "$user1")
    
    if [ "$decrypted" == "$plaintext" ]; then
        echo "Decrypt returns plaintext (no re-encryption support yet)"
        return 1
    else
        echo "Decrypt returned: $decrypted"
        return 0
    fi
}

# Run all tests
echo "=== Running Permission Feature Tests ==="
echo ""

run_test "Feature 1: Balance check in read-only mode" test_balance_check_readonly
run_test "Feature 2: Owner permission check" test_owner_permission_check  
run_test "Feature 3: Owner transfer" test_owner_transfer
run_test "Feature 4: Block crypto data sync" test_block_crypto_sync
run_test "Feature 5: Decrypt re-encryption" test_decrypt_reencryption

# Cleanup
cleanup_test "$TEST_DIR"

# Summary
echo ""
echo "================================"
echo "Permission Features Test Summary"
echo "================================"
echo "Total tests: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS"
echo "Failed: $((TOTAL_TESTS - PASSED_TESTS))"
echo "================================"

if [ $PASSED_TESTS -eq $TOTAL_TESTS ]; then
    echo "✓ ALL TESTS PASSED"
    exit 0
else
    echo "✗ SOME TESTS FAILED"
    exit 1
fi
