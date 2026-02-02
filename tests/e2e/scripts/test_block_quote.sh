#!/bin/bash
# E2E Test: Block Quote Attestation
# Tests SGX Quote generation and verification in blocks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../framework/test_env.sh"
source "$SCRIPT_DIR/../framework/assertions.sh"
source "$SCRIPT_DIR/../framework/node.sh"
source "$SCRIPT_DIR/../framework/contracts.sh"

TEST_NAME="Block Quote Attestation"
echo "========================================="
echo "E2E Test: $TEST_NAME"
echo "========================================="
echo ""

# Setup
TEST_DIR=$(setup_test_dir "block_quote")
print_test_env

# Get genesis file
GENESIS_FILE="$SCRIPT_DIR/../data/genesis.json"

# Initialize and start node
init_test_node "$TEST_DIR" "$GENESIS_FILE"
assert_success "Node initialization"

start_test_node "$TEST_DIR" 30311 8553

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0

# Helper function
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

# Test 1: Block contains Quote in Extra field
test_block_has_quote() {
    echo "Checking if blocks contain SGX Quote..."
    
    local block_number=$(get_block_number)
    echo "Current block number: $block_number"
    
    # Get block by number
    local block=$(call_rpc "eth_getBlockByNumber" "[\"0x$block_number\", false]")
    
    # Check if Extra field exists and is not empty
    local extra=$(echo "$block" | jq -r '.result.extraData')
    
    if [ "$extra" != "null" ] && [ "$extra" != "0x" ] && [ -n "$extra" ]; then
        echo "Block has Extra data: ${extra:0:66}..."
        return 0
    else
        echo "Block has no Extra data (Quote should be here)"
        return 1
    fi
}

# Test 2: Quote userData contains block hash
test_quote_userdata_block_hash() {
    echo "Testing Quote userData contains block hash..."
    
    # This test requires:
    # 1. Get a block
    # 2. Extract Quote from Extra field
    # 3. Parse Quote to get userData
    # 4. Verify userData == block hash
    
    # TODO: Need Quote parsing implementation
    echo "Quote parsing not yet implemented in test"
    return 1
}

# Test 3: Invalid Quote is rejected
test_invalid_quote_rejected() {
    echo "Testing invalid Quote rejection..."
    
    # This would require:
    # 1. Create a block with tampered Quote
    # 2. Submit to node
    # 3. Verify node rejects it
    
    # TODO: Needs block submission capability
    echo "Block submission test not implemented"
    return 1
}

# Test 4: Quote verification on sync
test_quote_verification_sync() {
    echo "Testing Quote verification during sync..."
    
    # This requires:
    # 1. Start second node
    # 2. Connect nodes for sync
    # 3. Verify Quote is checked during sync
    
    echo "Multi-node sync test not implemented"
    return 1
}

# Test 5: Quote proves MRENCLAVE
test_quote_mrenclave() {
    echo "Testing Quote contains MRENCLAVE..."
    
    # Check if Quote can be parsed to extract MRENCLAVE
    # TODO: Need Quote parsing
    echo "MRENCLAVE extraction not implemented"
    return 1
}

# Run all tests
echo "=== Running Block Quote Tests ==="
echo ""

run_test "Test 1: Block contains Quote" test_block_has_quote
run_test "Test 2: Quote userData = block hash" test_quote_userdata_block_hash
run_test "Test 3: Invalid Quote rejected" test_invalid_quote_rejected
run_test "Test 4: Quote verified on sync" test_quote_verification_sync
run_test "Test 5: Quote contains MRENCLAVE" test_quote_mrenclave

# Cleanup
cleanup_test "$TEST_DIR"

# Summary
echo ""
echo "================================"
echo "Block Quote Test Summary"
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
