# X Chain PoA-SGX End-to-End Tests

This directory contains comprehensive end-to-end tests for the X Chain PoA-SGX consensus implementation.

## Test Architecture

**Important**: Production code contains NO mock logic. Tests work by mocking the environment (files, not code).

### How Testing Works

1. **Mock Attestation Device**: Creates `/dev/attestation/*` files simulating SGX hardware
2. **Mock Manifest**: Provides security config contract address
3. **Environment Variables**: Represent genesis alloc storage (whitelist data)
4. **Production Code**: Runs unmodified, reads from mock environment

### Mock vs Production

- ❌ **No** `XCHAIN_SGX_MODE=mock` checks in code
- ✅ **Yes** `SGX_TEST_MODE=true` to skip hardware validation
- ✅ Production code reads from `/dev/attestation/` files
- ✅ Tests create mock files before running geth

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

Tests run with **environment mocking** (not code mocking). The framework creates:

1. **Mock `/dev/attestation/` files**:
   - `my_target_info`: Contains mock MRENCLAVE (sha256("mock-mrenclave-for-testing"))
   - `user_report_data`: Writable file for quote generation input
   - `quote`: Output file for generated quotes

2. **Mock manifest file** (`.manifest.sgx`):
   - Contains security config contract address
   - Located in test data directory
   - Points to genesis contract

3. **Environment variables** (represent genesis storage):
   - `XCHAIN_CONTRACT_MRENCLAVES`: Whitelist from contract storage
   - `XCHAIN_CONTRACT_MRSIGNERS`: Whitelist from contract storage

Key environment variables (configured in `framework/test_env.sh`):
- `GRAMINE_VERSION="1.0-test"` - Gramine version identifier
- `SGX_TEST_MODE="true"` - Skip hardware checks
- `XCHAIN_CONTRACT_MRENCLAVES` - Pre-configured whitelist
- `XCHAIN_CONTRACT_MRSIGNERS` - Pre-configured whitelist
- `INTEL_SGX_API_KEY` - Intel SGX API key for quote verification

## Mock Environment Details

### Mock Attestation Device
The test framework creates `/dev/attestation/*` to simulate Gramine's SGX interface:

```bash
# Mock MRENCLAVE (32 bytes)
echo "40807cade135f3346f59c3b40a45b8cf0ecc262e1b172afc62b82232e662c78a" | xxd -r -p > /dev/attestation/my_target_info

# Writable user_report_data
touch /dev/attestation/user_report_data

# Quote output file
touch /dev/attestation/quote
```

### Mock Manifest
Contains the security configuration contract address:

```toml
# geth.manifest.sgx
loader.env.XCHAIN_SECURITY_CONFIG_CONTRACT = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
```

### Genesis Storage Representation
Environment variables represent contract storage entries:

```bash
# These represent: mapping(bytes32 => bool) allowedMREnclaves
export XCHAIN_CONTRACT_MRENCLAVES="40807ca...,another..."

# These represent: mapping(bytes32 => bool) allowedMRSigners  
export XCHAIN_CONTRACT_MRSIGNERS="68192bc..."
```

## Troubleshooting

### Tests fail with "API key not set"
Set the `INTEL_SGX_API_KEY` environment variable before running tests.

### Tests fail with "geth binary not found"
Build geth first: `make geth` from the project root.

### Permission denied on /dev/attestation
The test framework uses `sudo` to create mock files in `/dev/attestation`. Ensure you have sudo privileges.

### Node fails to start
Check that ports 8545-8550 and 30303 are not already in use.

### Whitelist is empty
Ensure `XCHAIN_CONTRACT_MRENCLAVES` and `XCHAIN_CONTRACT_MRSIGNERS` are set in test environment.

## Test Coverage

The E2E test suite covers:
- ✅ SGX consensus engine initialization
- ✅ Manifest verification (signature + measurements)
- ✅ Whitelist loading from contract storage
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
6. **Important**: Mock the environment, not the code
