#!/bin/bash
# E2E Test: Read-Only Cryptographic Operations
# Tests read-only operations like getting public keys and verifying signatures

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRAMEWORK_DIR="$SCRIPT_DIR/../framework"
DATA_DIR="$SCRIPT_DIR/../data"

# Source framework files
source "$FRAMEWORK_DIR/assertions.sh"
source "$FRAMEWORK_DIR/test_env.sh"
source "$FRAMEWORK_DIR/node.sh"
source "$FRAMEWORK_DIR/contracts.sh"
source "$FRAMEWORK_DIR/crypto.sh"

# Test configuration
TEST_NAME="crypto_readonly"
DATADIR=""
RPC_PORT=8546

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
    echo "E2E Test: Read-Only Crypto Operations"
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
    
    # Initialize node
    init_test_node "$DATADIR" "$DATA_DIR/genesis.json"
    assert_success "Node initialization"
    
    # Start test node
    start_test_node "$DATADIR" 30304 "$RPC_PORT"
    assert_success "Node started"
    
    sleep 3
    
    # Define test accounts
    local USER1="0x1000000000000000000000000000000000000001"
    local USER2="0x2000000000000000000000000000000000000002"
    
    test_section "Test 1: Get Public Key (Read-Only)"
    
    # User1 creates a key
    local KEY_ID=$(sgx_create_key $KEY_TYPE_ECDSA "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$KEY_ID" "User creates ECDSA key"
    
    sleep 2
    
    # User1 gets their own public key (read-only)
    local PUBLIC_KEY_OWNER=$(sgx_get_public_key "$KEY_ID" "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$PUBLIC_KEY_OWNER" "Owner can read their public key"
    
    # User2 also gets the public key (read-only, should work)
    local PUBLIC_KEY_OTHER=$(sgx_get_public_key "$KEY_ID" "$USER2" "$RPC_PORT" 2>&1)
    assert_not_empty "$PUBLIC_KEY_OTHER" "Other users can read public key"
    
    # Verify both got the same public key
    if [ "$PUBLIC_KEY_OWNER" = "$PUBLIC_KEY_OTHER" ]; then
        echo -e "${GREEN}✓ PASS${NC}: Public key read is consistent across users"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Public key read inconsistent"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 2: Signature Verification (Read-Only)"
    
    # Create a message to sign
    local MESSAGE="0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
    
    # User1 signs the message
    local SIGNATURE=$(sgx_sign "$KEY_ID" "$MESSAGE" "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$SIGNATURE" "User signs message"
    
    sleep 2
    
    # User2 verifies the signature (read-only operation)
    local VERIFY_RESULT=$(sgx_verify "$PUBLIC_KEY_OWNER" "$MESSAGE" "$SIGNATURE" "$USER2" "$RPC_PORT" 2>&1 || echo "false")
    
    if [[ "$VERIFY_RESULT" == *"true"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}: Anyone can verify signatures (read-only)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Signature verification failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 3: Invalid Signature Verification"
    
    # Create an invalid signature (modify one byte)
    local INVALID_SIG="${SIGNATURE:0:10}00${SIGNATURE:12}"
    
    # Try to verify invalid signature
    local VERIFY_INVALID=$(sgx_verify "$PUBLIC_KEY_OWNER" "$MESSAGE" "$INVALID_SIG" "$USER2" "$RPC_PORT" 2>&1 || echo "false")
    
    if [[ "$VERIFY_INVALID" == *"false"* ]] || [[ "$VERIFY_INVALID" == *"FAIL"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}: Invalid signature correctly rejected"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Invalid signature incorrectly accepted"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 4: Random Number Generation (Read-Only)"
    
    # Generate random bytes
    local RANDOM1=$(sgx_random 32 "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$RANDOM1" "Can generate random bytes"
    
    # Generate again
    local RANDOM2=$(sgx_random 32 "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$RANDOM2" "Can generate random bytes again"
    
    # Verify they're different (randomness check)
    if [ "$RANDOM1" != "$RANDOM2" ]; then
        echo -e "${GREEN}✓ PASS${NC}: Random number generator produces different values"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Random number generator produced same value twice"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 5: Multiple Key Types Public Key Retrieval"
    
    # Create Ed25519 key
    local ED25519_KEY=$(sgx_create_key $KEY_TYPE_ED25519 "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$ED25519_KEY" "Can create Ed25519 key"
    
    sleep 2
    
    # Get Ed25519 public key
    local ED25519_PUBLIC=$(sgx_get_public_key "$ED25519_KEY" "$USER1" "$RPC_PORT" 2>&1)
    assert_not_empty "$ED25519_PUBLIC" "Can get Ed25519 public key"
    
    # Verify it's different from ECDSA key
    if [ "$ED25519_PUBLIC" != "$PUBLIC_KEY_OWNER" ]; then
        echo -e "${GREEN}✓ PASS${NC}: Different key types have different public keys"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Key types have same public key"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    # Print test summary
    print_test_summary "Read-Only Crypto Operations Tests"
}

# Run main function
main

# Exit with appropriate code
if [ $TESTS_FAILED -eq 0 ]; then
    exit 0
else
    exit 1
fi
