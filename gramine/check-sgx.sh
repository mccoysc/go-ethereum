#!/bin/bash
# check-sgx.sh
# Check SGX hardware support and driver installation

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== SGX Hardware Support Check ===${NC}"
echo ""

# Check CPU model
echo -e "${BLUE}CPU Model:${NC}"
lscpu | grep "Model name" || echo "Could not detect CPU model"
echo ""

# Check for SGX support via cpuid (if available)
echo -e "${BLUE}Checking SGX Support:${NC}"
if command -v cpuid &> /dev/null; then
    if cpuid | grep -q SGX; then
        echo -e "${GREEN}✓ CPU supports SGX${NC}"
    else
        echo -e "${RED}✗ CPU does not support SGX${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠ cpuid command not found, skipping CPU SGX check${NC}"
    echo "  Install with: sudo apt install cpuid"
fi
echo ""

# Check SGX devices
echo -e "${BLUE}Checking SGX Devices:${NC}"

if [ -c /dev/sgx_enclave ]; then
    echo -e "${GREEN}✓ SGX enclave device found (/dev/sgx_enclave)${NC}"
else
    echo -e "${RED}✗ SGX enclave device not found (/dev/sgx_enclave)${NC}"
    echo "  Please install SGX driver"
    exit 1
fi

if [ -c /dev/sgx_provision ]; then
    echo -e "${GREEN}✓ SGX provision device found (/dev/sgx_provision)${NC}"
else
    echo -e "${RED}✗ SGX provision device not found (/dev/sgx_provision)${NC}"
    echo "  This is required for DCAP attestation"
    exit 1
fi
echo ""

# Check AESM service
echo -e "${BLUE}Checking AESM Service:${NC}"
if pgrep -x "aesm_service" > /dev/null; then
    echo -e "${GREEN}✓ AESM service is running${NC}"
else
    echo -e "${RED}✗ AESM service is not running${NC}"
    echo "  Start with: sudo systemctl start aesmd"
    echo "  Or: sudo /opt/intel/sgx-aesm-service/aesm/aesm_service &"
    exit 1
fi
echo ""

# Check SGX driver version (if sgx-detect is available)
echo -e "${BLUE}SGX Driver Information:${NC}"
if command -v sgx-detect &> /dev/null; then
    sgx-detect
    echo ""
else
    echo -e "${YELLOW}⚠ sgx-detect not installed, skipping detailed driver check${NC}"
    echo "  This is optional but recommended for debugging"
fi

# Check Gramine installation
echo -e "${BLUE}Checking Gramine:${NC}"
if command -v gramine-sgx &> /dev/null; then
    echo -e "${GREEN}✓ Gramine is installed${NC}"
    gramine-sgx --version
else
    echo -e "${RED}✗ Gramine is not installed${NC}"
    echo "  Install from: https://gramine.readthedocs.io"
    exit 1
fi
echo ""

# Check for sgx-quote-dump (optional debug tool)
if command -v gramine-sgx-quote-dump &> /dev/null; then
    echo -e "${GREEN}✓ Gramine SGX quote dump utility available${NC}"
else
    echo -e "${YELLOW}⚠ gramine-sgx-quote-dump not found (optional)${NC}"
fi
echo ""

# Summary
echo -e "${GREEN}=== All Checks Passed ===${NC}"
echo ""
echo "Your system is ready for X Chain deployment with SGX support!"
echo ""
echo "Next steps:"
echo "  1. Build the X Chain node: cd gramine && ./build-docker.sh"
echo "  2. Or run locally for testing: cd gramine && ./run-dev.sh sgx"
