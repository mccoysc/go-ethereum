# X Chain End-to-End Tests

## Overview

This directory contains comprehensive end-to-end tests for the X Chain PoA-SGX implementation. These tests verify the complete functionality of:

1. **SGX Cryptographic Interfaces** (Precompiled Contracts 0x8000-0x80FF)
   - Key creation and management
   - Owner logic and permissions
   - Signature operations
   - Encryption/decryption
   - Key derivation

2. **Governance Contracts**
   - Bootstrap contract and founder registration
   - MRENCLAVE whitelist management
   - Voting mechanisms
   - Validator management

3. **Consensus Mechanisms**
   - On-demand block production
   - Multi-producer rewards
   - Reputation and penalty systems

## Test Structure

```
tests/e2e/
├── README.md                          # This file
├── framework/                         # Test framework utilities
│   ├── node.sh                        # Node management functions
│   ├── contracts.sh                   # Contract interaction utilities
│   ├── crypto.sh                      # Cryptographic test utilities
│   └── assertions.sh                  # Test assertion helpers
├── scripts/                           # Individual test scripts
│   ├── test_crypto_owner.sh          # Owner logic tests
│   ├── test_crypto_deploy.sh         # Contract deployment tests
│   ├── test_crypto_readonly.sh       # Read-only operation tests
│   ├── test_governance_bootstrap.sh  # Bootstrap contract tests
│   ├── test_governance_whitelist.sh  # Whitelist management tests
│   ├── test_governance_voting.sh     # Voting mechanism tests
│   └── test_consensus_production.sh  # Block production tests
├── data/                              # Test data and fixtures
│   ├── genesis.json                   # Test genesis configuration
│   └── test_accounts.json             # Pre-funded test accounts
└── run_all_tests.sh                   # Main test runner

```

## Running Tests

### Prerequisites
- Built `geth` binary in `build/bin/`
- Node.js and web3.js installed (for contract interactions)
- jq installed (for JSON processing)

### Run All Tests
```bash
./tests/e2e/run_all_tests.sh
```

### Run Individual Test Suites
```bash
./tests/e2e/scripts/test_crypto_owner.sh
./tests/e2e/scripts/test_crypto_deploy.sh
./tests/e2e/scripts/test_governance_bootstrap.sh
```

## Test Requirements

All tests must:
- Start fresh blockchain nodes
- Execute transactions and verify results
- Check state changes on-chain
- Clean up resources after completion
- Report PASS/FAIL clearly

## Test Coverage

### Cryptographic Interface Tests
- [ ] Key creation (ECDSA, Ed25519, AES-256)
- [ ] Owner permission checks
- [ ] Key deletion by owner
- [ ] Signature creation and verification
- [ ] Encryption and decryption
- [ ] ECDH key exchange
- [ ] Key derivation
- [ ] Random number generation
- [ ] Read-only public key retrieval

### Governance Tests
- [ ] Bootstrap founder registration
- [ ] MRENCLAVE whitelist add/remove
- [ ] Proposal creation and voting
- [ ] Validator admission
- [ ] Validator stake management

### Consensus Tests
- [ ] On-demand block production
- [ ] Transaction processing
- [ ] Block reward distribution
- [ ] Reputation tracking

## Exit Codes
- 0: All tests passed
- 1: One or more tests failed
- 2: Setup/environment error
