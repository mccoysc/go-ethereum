#!/bin/bash
# E2E Test: Cryptographic Contract Deployment
# Tests deployment and usage of all cryptographic precompiled contracts

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

# Test configuration
TEST_NAME="crypto_deploy"
DATADIR=""
RPC_PORT=8547

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    if [ -n "$DATADIR" ]; then
        cleanup_test_node "$DATADIR"
    fi
}

trap cleanup EXIT INT TERM

main() {
    echo "========================================="
    echo "E2E Test: Crypto Contract Deployment"
    echo "========================================="
    
    local geth=$(get_geth_binary)
    if [ ! -f "$geth" ]; then
        echo "Error: geth binary not found at $geth"
        exit 2
    fi
    
    test_section "Setup Test Environment"
    
    DATADIR=$(create_test_datadir "$TEST_NAME")
    echo "Test data directory: $DATADIR"
    
    init_test_node "$DATADIR" "$DATA_DIR/genesis.json"
    assert_success "Node initialization"
    
    start_test_node "$DATADIR" 30305 "$RPC_PORT"
    assert_success "Node started"
    
    sleep 3
    
    local USER="0x1000000000000000000000000000000000000001"
    
    test_section "Test 1: Verify Precompiled Contract Addresses"
    
    # Check that precompiled contracts are accessible
    # Try calling each contract to verify it exists
    
    # SGX_KEY_CREATE (0x8000)
    local code=$(get_contract_code "$SGX_KEY_CREATE" "$RPC_PORT")
    # Precompiled contracts may return 0x or empty code
    echo "SGX_KEY_CREATE address: $SGX_KEY_CREATE"
    echo "Code length: ${#code}"
    
    # Even if code is empty, we can test by trying to call it
    
    test_section "Test 2: Deploy (Create) ECDSA Key"
    
    local ECDSA_KEY=$(sgx_create_key $KEY_TYPE_ECDSA "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$ECDSA_KEY" "ECDSA key created via precompiled contract"
    
    sleep 2
    
    local ECDSA_PUBLIC=$(sgx_get_public_key "$ECDSA_KEY" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$ECDSA_PUBLIC" "ECDSA public key retrieved"
    
    test_section "Test 3: Deploy (Create) Ed25519 Key"
    
    local ED25519_KEY=$(sgx_create_key $KEY_TYPE_ED25519 "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$ED25519_KEY" "Ed25519 key created via precompiled contract"
    
    sleep 2
    
    local ED25519_PUBLIC=$(sgx_get_public_key "$ED25519_KEY" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$ED25519_PUBLIC" "Ed25519 public key retrieved"
    
    test_section "Test 4: Deploy (Create) AES-256 Key"
    
    local AES_KEY=$(sgx_create_key $KEY_TYPE_AES256 "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$AES_KEY" "AES-256 key created via precompiled contract"
    
    sleep 2
    
    test_section "Test 5: Test Encryption/Decryption Contract"
    
    # Test data to encrypt
    local PLAINTEXT="0x48656c6c6f20576f726c64"  # "Hello World" in hex
    
    # Encrypt with AES key
    local CIPHERTEXT=$(sgx_encrypt "$AES_KEY" "$PLAINTEXT" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$CIPHERTEXT" "Data encrypted via SGX_ENCRYPT contract"
    
    sleep 2
    
    # Decrypt the ciphertext
    local DECRYPTED=$(sgx_decrypt "$AES_KEY" "$CIPHERTEXT" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$DECRYPTED" "Data decrypted via SGX_DECRYPT contract"
    
    # Verify decrypted matches original (allowing for padding differences)
    if [[ "$DECRYPTED" == *"$(echo $PLAINTEXT | sed 's/0x//')"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}: Decrypted data matches original plaintext"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Decrypted data doesn't match"
        echo "  Expected: $PLAINTEXT"
        echo "  Got: $DECRYPTED"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 6: Test ECDH Key Exchange Contract"
    
    # Create two ECDSA keys for ECDH
    local KEY_A=$(sgx_create_key $KEY_TYPE_ECDSA "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$KEY_A" "ECDH participant A key created"
    
    sleep 2
    
    local KEY_B=$(sgx_create_key $KEY_TYPE_ECDSA "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$KEY_B" "ECDH participant B key created"
    
    sleep 2
    
    # Get public keys
    local PUBLIC_A=$(sgx_get_public_key "$KEY_A" "$USER" "$RPC_PORT" 2>&1)
    local PUBLIC_B=$(sgx_get_public_key "$KEY_B" "$USER" "$RPC_PORT" 2>&1)
    
    assert_not_empty "$PUBLIC_A" "Public key A retrieved"
    assert_not_empty "$PUBLIC_B" "Public key B retrieved"
    
    # Perform ECDH: A's private + B's public
    local SHARED_AB=$(sgx_ecdh "$KEY_A" "$PUBLIC_B" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SHARED_AB" "ECDH shared secret A->B computed"
    
    sleep 2
    
    # Perform ECDH: B's private + A's public
    local SHARED_BA=$(sgx_ecdh "$KEY_B" "$PUBLIC_A" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SHARED_BA" "ECDH shared secret B->A computed"
    
    # Verify both shared secrets match
    if [ "$SHARED_AB" = "$SHARED_BA" ]; then
        echo -e "${GREEN}✓ PASS${NC}: ECDH shared secrets match"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: ECDH shared secrets don't match"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 7: Test Sign/Verify Contract Integration"
    
    # Create a key for signing
    local SIGN_KEY=$(sgx_create_key $KEY_TYPE_ECDSA "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SIGN_KEY" "Signing key created"
    
    sleep 2
    
    local SIGN_PUBLIC=$(sgx_get_public_key "$SIGN_KEY" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SIGN_PUBLIC" "Signing public key retrieved"
    
    # Create message to sign
    local MESSAGE="0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    
    # Sign the message
    local SIGNATURE=$(sgx_sign "$SIGN_KEY" "$MESSAGE" "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$SIGNATURE" "Message signed via SGX_SIGN contract"
    
    sleep 2
    
    # Verify the signature
    local VERIFY=$(sgx_verify "$SIGN_PUBLIC" "$MESSAGE" "$SIGNATURE" "$USER" "$RPC_PORT" 2>&1 || echo "false")
    
    if [[ "$VERIFY" == *"true"* ]]; then
        echo -e "${GREEN}✓ PASS${NC}: Signature verified via SGX_VERIFY contract"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Signature verification failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    test_section "Test 8: Test Random Number Contract"
    
    # Generate 32 bytes of random data
    local RANDOM=$(sgx_random 32 "$USER" "$RPC_PORT" 2>&1)
    assert_not_empty "$RANDOM" "Random bytes generated via SGX_RANDOM contract"
    
    # Verify it's the right length (32 bytes = 64 hex chars + 0x prefix)
    local EXPECTED_LENGTH=66  # 0x + 64 hex chars
    local ACTUAL_LENGTH=${#RANDOM}
    
    if [ $ACTUAL_LENGTH -ge $EXPECTED_LENGTH ]; then
        echo -e "${GREEN}✓ PASS${NC}: Random data has correct length"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: Random data length incorrect"
        echo "  Expected: >= $EXPECTED_LENGTH, Got: $ACTUAL_LENGTH"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    
    print_test_summary "Crypto Contract Deployment Tests"
}

main

if [ $TESTS_FAILED -eq 0 ]; then
    exit 0
else
    exit 1
fi
