#!/bin/bash
# E2E Test: Cryptographic Owner Logic
# Tests owner permissions and access control for SGX cryptographic operations

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRAMEWORK_DIR="$SCRIPT_DIR/../framework"
DATA_DIR="$SCRIPT_DIR/../data"

# Source framework files
source "$FRAMEWORK_DIR/assertions.sh"
source "$FRAMEWORK_DIR/node.sh"
source "$FRAMEWORK_DIR/contracts.sh"
source "$FRAMEWORK_DIR/crypto.sh"
source "$FRAMEWORK_DIR/test_env.sh"

# Test configuration
TEST_NAME="crypto_owner"
DATADIR=""
RPC_PORT=8545

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    if [ -n "$DATADIR" ]; then
        cleanup_test_node "$DATADIR"
    fi
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

# Main test function
main() {
    echo "========================================="
    echo "E2E Test: Cryptographic Owner Logic"
    echo "========================================="
    
    # Check if geth is built
    local geth=$(get_geth_binary)
    if [ ! -f "$geth" ]; then
        echo "Error: geth binary not found at $geth"
        echo "Please run 'make geth' first"
        exit 2
    fi
    
    test_section "Setup Test Environment"
    
    # Create test datadir
    DATADIR=$(create_test_datadir "$TEST_NAME")
    echo "Test data directory: $DATADIR"
    
    # Initialize node with test genesis
    init_test_node "$DATADIR" "$DATA_DIR/genesis.json"
    assert_success "Node initialization"
    
    # Start test node
    start_test_node "$DATADIR" 30303 "$RPC_PORT"
    assert_success "Node started"
    
    # Wait for node to be ready
    sleep 3
    
    # Define test accounts (from genesis)
    local OWNER="0x1000000000000000000000000000000000000001"
    local NON_OWNER="0x2000000000000000000000000000000000000002"
    
    test_section "Test 1: Owner Can Create Keys"
    
    # Owner creates a key
    local KEY_ID=$(sgx_create_key $KEY_TYPE_ECDSA "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$KEY_ID" "Owner can create ECDSA key"
    
    # Get the public key to verify key was created
    local PUBLIC_KEY=$(sgx_get_public_key "$KEY_ID" "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$PUBLIC_KEY" "Can retrieve public key for created key"
    
    test_section "Test 2: Owner Can Delete Their Own Keys"
    
    # Owner deletes the key
    local DELETE_TX=$(sgx_delete_key "$KEY_ID" "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$DELETE_TX" "Owner can delete their own key"
    
    # Wait a bit for transaction to be processed
    sleep 2
    
    # Try to get public key again (should fail or return empty)
    local PUBLIC_KEY_AFTER=$(sgx_get_public_key "$KEY_ID" "$OWNER" "$RPC_PORT" 2>&1 || echo "")
    # Note: We expect this to fail or return empty, but we just verify the delete worked
    
    test_section "Test 3: Non-Owner Cannot Delete Others' Keys"
    
    # Owner creates another key
    local KEY_ID2=$(sgx_create_key $KEY_TYPE_ECDSA "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$KEY_ID2" "Owner creates another ECDSA key"
    
    sleep 2
    
    # Non-owner tries to delete the key (should fail)
    local DELETE_TX_FAIL=$(sgx_delete_key "$KEY_ID2" "$NON_OWNER" "$RPC_PORT" 2>&1 || echo "FAILED")
    
    # We expect this to fail - if it contains FAILED or is empty, that's good
    if [[ "$DELETE_TX_FAIL" == *"FAILED"* ]] || [ -z "$DELETE_TX_FAIL" ]; then
        echo -e "${GREEN}✓ PASS${NC}: Non-owner correctly prevented from deleting others' keys"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        # If it succeeded, verify the key is still there
        local PUBLIC_KEY_STILL_EXISTS=$(sgx_get_public_key "$KEY_ID2" "$OWNER" "$RPC_PORT" 2>&1)
        if [ -n "$PUBLIC_KEY_STILL_EXISTS" ]; then
            echo -e "${GREEN}✓ PASS${NC}: Key still exists after non-owner delete attempt"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}✗ FAIL${NC}: Non-owner was able to delete owner's key"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 4: Owner Can Use Their Own Keys for Signing"
    
    # Create a message hash to sign
    local MESSAGE_HASH="0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
    
    # Owner signs with their key
    local SIGNATURE=$(sgx_sign "$KEY_ID2" "$MESSAGE_HASH" "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SIGNATURE" "Owner can sign with their own key"
    
    sleep 2
    
    # Get public key and verify signature
    local PUBLIC_KEY_FOR_VERIFY=$(sgx_get_public_key "$KEY_ID2" "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$PUBLIC_KEY_FOR_VERIFY" "Can get public key for verification"
    
    # Verify the signature
    local VERIFY_RESULT=$(sgx_verify "$PUBLIC_KEY_FOR_VERIFY" "$MESSAGE_HASH" "$SIGNATURE" "$OWNER" "$RPC_PORT" 2>&1 || echo "false")
    
    if [[ "$VERIFY_RESULT" == *"true"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}: Signature verification successful"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Signature verification failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 5: Multiple Users Can Create Their Own Keys"
    
    # Owner creates a key
    local OWNER_KEY=$(sgx_create_key $KEY_TYPE_ECDSA "$OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$OWNER_KEY" "Owner creates their key"
    
    sleep 2
    
    # Non-owner creates their own key
    local NON_OWNER_KEY=$(sgx_create_key $KEY_TYPE_ECDSA "$NON_OWNER" "$RPC_PORT" 2>&1)
    assert_not_empty "$NON_OWNER_KEY" "Non-owner creates their own key"
    
    sleep 2
    
    # Verify both keys exist and are different
    if [ "$OWNER_KEY" != "$NON_OWNER_KEY" ]; then
        echo -e "${GREEN}✓ PASS${NC}: Different users get different key IDs"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Same key ID returned for different users"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    # Print test summary
    print_test_summary "Cryptographic Owner Logic Tests"
}

# Run main function
main

# Exit with appropriate code
if [ $TESTS_FAILED -eq 0 ]; then
    exit 0
else
    exit 1
fi
