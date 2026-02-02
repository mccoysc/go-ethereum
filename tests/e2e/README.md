# X Chain PoA-SGX End-to-End Tests

This directory contains comprehensive end-to-end tests for the X Chain PoA-SGX consensus implementation.

## Prerequisites

1. **Build geth:**
   ```bash
   cd /path/to/go-ethereum
   make geth
   ```

2. **Install jq (optional but recommended):**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install jq
   
   # macOS
   brew install jq
   ```

3. **Set Intel SGX API Key (required for quote verification):**
   
   The tests require an Intel SGX API key for quote verification via Intel's Platform Certification Caching Service (PCCS).
   
   **Option 1: Use default test key (already configured)**
   - The default test key `a8ece8747e7b4d8d98d23faec065b0b8` is pre-configured in `framework/test_env.sh`
   
   **Option 2: Use your own key**
   ```bash
   export INTEL_SGX_API_KEY="your-api-key-here"
   ```
   
   **To obtain your own API key:**
   - Visit https://api.portal.trustedservices.intel.com/
   - Register and obtain your subscription key
   - Set the environment variable before running tests

## Running Tests

### Run all test suites:
```bash
cd tests/e2e
./run_all_tests.sh
```

### Run individual test suites:
```bash
# Consensus block production tests
./scripts/test_consensus_production.sh

# Cryptographic operations tests
./scripts/test_crypto_owner.sh
./scripts/test_crypto_readonly.sh
./scripts/test_crypto_deploy.sh

# Permission and access control tests
./scripts/test_permissions.sh

# Block quote attestation tests
./scripts/test_block_quote.sh

# Governance contract tests
./scripts/test_governance.sh
```

## Test Categories

### 1. Consensus Tests (`test_consensus_production.sh`)
- On-demand block production
- Transaction batching
- Heartbeat blocks (periodic block generation for miner incentives)
- Block validation

### 2. Cryptographic Interface Tests
- **Owner logic** (`test_crypto_owner.sh`): Key creation, deletion, ownership
- **Read-only operations** (`test_crypto_readonly.sh`): Public key retrieval, signature verification
- **Contract deployment** (`test_crypto_deploy.sh`): Full integration tests

### 3. Permission Tests (`test_permissions.sh`)
- Balance checks for key creation
- Owner permission verification
- Read-only mode restrictions

### 4. Block Quote Tests (`test_block_quote.sh`)
- SGX Quote generation with block hash
- Quote verification on block sync
- Platform Instance ID extraction
- Attestation validation

### 5. Governance Tests (`test_governance.sh`)
- Bootstrap contract interactions
- Whitelist management (MRENCLAVE/MRSIGNER)
- Validator governance
- Voting mechanisms

## Test Environment

The tests run in mock SGX mode (`XCHAIN_SGX_MODE=mock`) which simulates the SGX environment without requiring actual SGX hardware.

Key environment variables (configured in `framework/test_env.sh`):
- `XCHAIN_SGX_MODE=mock` - Enable mock SGX mode for testing
- `XCHAIN_GOVERNANCE_CONTRACT` - Pre-deployed governance contract address
- `XCHAIN_SECURITY_CONFIG_CONTRACT` - Pre-deployed security config contract address
- `INTEL_SGX_API_KEY` - Intel SGX API key for quote verification
- `GRAMINE_VERSION` - Gramine version identifier

## Mock Environment

The test framework sets up a complete mock environment including:

1. **Mock /dev/attestation device** - Simulates Gramine's attestation interface
2. **Mock SGX quotes** - Generates valid quote structures for testing
3. **Mock manifest files** - For signature verification testing
4. **Pre-deployed contracts** - Governance and security configuration contracts

## Troubleshooting

### Tests fail with "API key not set"
Set the `INTEL_SGX_API_KEY` environment variable before running tests.

### Tests fail with "geth binary not found"
Build geth first: `make geth` from the project root.

### Permission denied on /dev/attestation
The test framework uses `sudo` to create mock files in `/dev/attestation`. Ensure you have sudo privileges.

### Node fails to start
Check that ports 8545-8550 and 30303 are not already in use.

## Test Coverage

The E2E test suite covers:
- ✅ SGX consensus engine initialization
- ✅ Block production (on-demand and heartbeat)
- ✅ All 10 SGX precompiled contracts (0x8000-0x8009)
- ✅ Quote generation and verification
- ✅ Owner permission controls
- ✅ Transaction execution
- ✅ State management
- ⏳ Governance contract interactions (partial)
- ⏳ Multi-node consensus (planned)

## Contributing

When adding new tests:
1. Place test scripts in `scripts/` directory
2. Source framework files from `framework/`
3. Use consistent naming: `test_<category>_<feature>.sh`
4. Add cleanup handlers for proper resource cleanup
5. Update this README with new test descriptions
