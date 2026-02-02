#!/bin/bash
# Main E2E Test Runner for X Chain PoA-SGX
# Runs all end-to-end tests and reports results

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$SCRIPT_DIR/scripts"

# Test results
TOTAL_SUITES=0
PASSED_SUITES=0
FAILED_SUITES=0

# Print header
print_header() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                                                            ║${NC}"
    echo -e "${BLUE}║          X Chain PoA-SGX End-to-End Test Suite            ║${NC}"
    echo -e "${BLUE}║                                                            ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Run a single test suite
run_test_suite() {
    local test_name="$1"
    local test_script="$2"
    
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Running: $test_name${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    TOTAL_SUITES=$((TOTAL_SUITES + 1))
    
    if bash "$test_script"; then
        echo -e "${GREEN}✓ SUITE PASSED: $test_name${NC}"
        PASSED_SUITES=$((PASSED_SUITES + 1))
        return 0
    else
        echo -e "${RED}✗ SUITE FAILED: $test_name${NC}"
        FAILED_SUITES=$((FAILED_SUITES + 1))
        return 1
    fi
}

# Print final summary
print_summary() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                      Test Summary                          ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Total test suites: $TOTAL_SUITES"
    echo -e "Passed: ${GREEN}$PASSED_SUITES${NC}"
    echo -e "Failed: ${RED}$FAILED_SUITES${NC}"
    echo ""
    
    if [ $FAILED_SUITES -eq 0 ]; then
        echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║                 🎉 ALL TESTS PASSED! 🎉                   ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
        return 0
    else
        echo -e "${RED}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║                  ❌ SOME TESTS FAILED ❌                   ║${NC}"
        echo -e "${RED}╚════════════════════════════════════════════════════════════╝${NC}"
        return 1
    fi
}

# Main function
main() {
    print_header
    
    # Check prerequisites
    echo "Checking prerequisites..."
    
    # Check for geth binary
    if [ ! -f "$SCRIPT_DIR/../../build/bin/geth" ]; then
        echo -e "${RED}Error: geth binary not found${NC}"
        echo "Please run 'make geth' from the project root first"
        exit 2
    fi
    echo -e "${GREEN}✓${NC} geth binary found"
    
    # Check for jq
    if ! command -v jq &> /dev/null; then
        echo -e "${YELLOW}Warning: jq not found${NC}"
        echo "Some tests may not work correctly without jq"
        echo "Install with: apt-get install jq (Debian/Ubuntu) or brew install jq (macOS)"
    else
        echo -e "${GREEN}✓${NC} jq found"
    fi
    
    # Check for curl
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}Error: curl not found${NC}"
        echo "curl is required for JSON-RPC calls"
        exit 2
    fi
    echo -e "${GREEN}✓${NC} curl found"
    
    echo ""
    echo "Starting E2E test suite..."
    
    # Run test suites
    run_test_suite "Cryptographic Owner Logic" "$SCRIPTS_DIR/test_crypto_owner.sh" || true
    run_test_suite "Read-Only Crypto Operations" "$SCRIPTS_DIR/test_crypto_readonly.sh" || true
    run_test_suite "Crypto Contract Deployment" "$SCRIPTS_DIR/test_crypto_deploy.sh" || true
    run_test_suite "Consensus Block Production" "$SCRIPTS_DIR/test_consensus_production.sh" || true
    run_test_suite "Permission Features" "$SCRIPTS_DIR/test_permissions.sh" || true
    run_test_suite "Block Quote Attestation" "$SCRIPTS_DIR/test_block_quote.sh" || true
    
    # Print summary
    print_summary
}

# Run main function
main

# Exit with appropriate code
if [ $FAILED_SUITES -eq 0 ]; then
    exit 0
else
    exit 1
fi
