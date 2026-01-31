# Module 01 Integration Guide

## Overview

Module 02 (SGX Consensus Engine) now integrates with Module 01 (SGX Attestation) to use the actual Intel SGX attestation implementation instead of mock interfaces.

## Changes Made

### 1. Updated Interfaces (`consensus/sgx/interfaces.go`)

The `Attestor` and `Verifier` interfaces now extend the interfaces from `internal/sgx`:

```go
// Attestor extends the internal/sgx.Attestor interface with consensus-specific methods
type Attestor interface {
	internalsgx.Attestor  // From Module 01

	// Consensus-specific methods
	SignInEnclave(data []byte) ([]byte, error)
	GetProducerID() ([]byte, error)
}

// Verifier extends the internal/sgx.Verifier interface with consensus-specific methods  
type Verifier interface {
	internalsgx.Verifier  // From Module 01

	// Consensus-specific methods
	VerifySignature(data, signature, producerID []byte) error
	ExtractProducerID(quote []byte) ([]byte, error)
}
```

### 2. Created Adapter Layer (`consensus/sgx/sgx_adapter.go`)

Adapter classes bridge Module 01's implementation with Module 02's needs:

- **AttestorAdapter**: Wraps `internalsgx.Attestor` and adds:
  - `SignInEnclave()` - Signs data using ECDSA with the node's private key
  - `GetProducerID()` - Returns the Ethereum address derived from the public key

- **VerifierAdapter**: Wraps `internalsgx.Verifier` and adds:
  - `VerifySignature()` - Verifies ECDSA signatures using recovered public key
  - `ExtractProducerID()` - Extracts producer ID from SGX quote's report data

### 3. Integration Helper (`consensus/sgx/module01_integration.go`)

Convenience functions to create the consensus engine with Module 01:

```go
// Create engine with Module 01
engine, err := NewWithModule01(config, privateKey)

// Create engine with MRENCLAVE whitelist
engine, err := NewWithModule01AndMRENCLAVEWhitelist(
    config, 
    privateKey, 
    allowedMREnclaves,
)
```

### 4. Updated Tests (`consensus/sgx/consensus_test.go`)

- Mock implementations updated to implement both interfaces
- Added `TestModule01Integration` to verify the integration works correctly
- Tests verify:
  - Quote generation
  - Producer ID extraction
  - Signature creation and verification
  - Quote verification

## Usage Examples

### Production Usage (with actual SGX hardware)

```go
import (
    "crypto/ecdsa"
    "github.com/ethereum/go-ethereum/consensus/sgx"
    "github.com/ethereum/go-ethereum/crypto"
)

// Generate or load your node's private key
privateKey, _ := crypto.GenerateKey()

// Create consensus config
config := sgx.DefaultConfig()

// Create engine with Module 01 integration
engine, err := sgx.NewWithModule01(config, privateKey)
if err != nil {
    log.Fatal(err)
}

// Use the engine
// engine.Author(header)
// engine.VerifyHeader(chain, header)
// etc.
```

### Development/Testing (without SGX hardware)

Module 01 automatically detects if it's running in an SGX environment and falls back to mock mode for testing:

```go
// Same code works in both environments
engine, err := sgx.NewWithModule01(config, privateKey)

// In non-SGX environment:
// - Uses mock SGX quotes
// - Uses deterministic test values for MRENCLAVE/MRSIGNER
// - Still verifies signatures properly
```

### Using Mock Implementation (for unit tests)

```go
// Create mock attestor and verifier
attestor := NewMockAttestor()
verifier := NewMockVerifier()

// Create engine with mocks
engine := sgx.New(config, attestor, verifier)

// Mocks always pass validation - useful for testing other components
```

## Architecture

```
┌─────────────────────────────────────┐
│   Module 02: Consensus Engine       │
│   (consensus/sgx)                   │
├─────────────────────────────────────┤
│  ┌──────────────────────────────┐  │
│  │ SGXEngine                     │  │
│  │ - Uses Attestor interface     │  │
│  │ - Uses Verifier interface     │  │
│  └──────────────────────────────┘  │
│               ▲                     │
│               │                     │
│  ┌────────────┴──────────────┐    │
│  │ Adapter Layer              │    │
│  │ - AttestorAdapter          │    │
│  │ - VerifierAdapter          │    │
│  └────────────┬──────────────┘    │
└───────────────┼───────────────────┘
                │
                ▼
┌─────────────────────────────────────┐
│   Module 01: SGX Attestation        │
│   (internal/sgx)                    │
├─────────────────────────────────────┤
│  ┌──────────────────────────────┐  │
│  │ GramineAttestor               │  │
│  │ - GenerateQuote()             │  │
│  │ - GenerateCertificate()       │  │
│  │ - GetMREnclave()              │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │ DCAPVerifier                  │  │
│  │ - VerifyQuote()               │  │
│  │ - VerifyCertificate()         │  │
│  │ - MRENCLAVE whitelist mgmt    │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
                │
                ▼
        ┌───────────────┐
        │ Gramine LibOS │
        │ Intel SGX     │
        └───────────────┘
```

## Benefits

1. **Real SGX Integration**: Uses actual Intel SGX attestation instead of mocks
2. **Automatic Fallback**: Works in both SGX and non-SGX environments
3. **Clean Separation**: Module 01 handles SGX details, Module 02 focuses on consensus
4. **Testability**: Mock implementations still available for unit tests
5. **Production Ready**: Can deploy with real SGX hardware

## Testing

Run all tests including Module 01 integration:

```bash
cd consensus/sgx
go test -v
```

Expected output:
```
=== RUN   TestNewEngine
--- PASS: TestNewEngine
=== RUN   TestModule01Integration
    Module 01 integration test passed successfully
--- PASS: TestModule01Integration
...
PASS
ok      github.com/ethereum/go-ethereum/consensus/sgx
```

## Security Considerations

### In Production (with real SGX)

- **Quote Verification**: Module 01 verifies SGX quotes using Intel DCAP
- **MRENCLAVE Whitelist**: Only allows known trusted enclaves to participate
- **TCB Status**: Checks that the enclave's Trusted Computing Base is up-to-date
- **Signature Verification**: Uses ECDSA with secp256k1 for Ethereum compatibility

### In Development (without SGX)

- Mock quotes are used for testing
- Signature verification still works (using real crypto)
- MRENCLAVE checks are bypassed (whitelist empty = allow all)
- Suitable for development and CI/CD pipelines

## Troubleshooting

### Build Errors

If you see errors about missing SGX libraries:
```
# This is normal - Module 01 has fallback implementations
# The code will compile and run in mock mode
```

### Runtime Errors

**Error**: "Failed to create engine with Module 01: failed to read MRENCLAVE"
- **Cause**: Not running in SGX environment
- **Solution**: This is expected; the code automatically falls back to mock mode

**Error**: "MRENCLAVE not in allowed list"
- **Cause**: MRENCLAVE whitelist is configured but doesn't include this enclave
- **Solution**: Add the MRENCLAVE to the whitelist or use `NewWithModule01()` which allows all in development

## Future Improvements

1. **Certificate-Based Authentication**: Use RA-TLS certificates for node-to-node communication
2. **Dynamic Whitelist Management**: Fetch MRENCLAVE whitelist from governance contract
3. **Instance ID Verification**: Prevent Sybil attacks using hardware-specific identifiers
4. **Performance Optimization**: Cache quote verification results to reduce overhead

## References

- Module 01 Documentation: `docs/modules/01-sgx-attestation.md`
- Module 02 Documentation: `docs/modules/02-consensus-engine.md`
- SGX Attestation Implementation: `internal/sgx/`
- Consensus Engine Implementation: `consensus/sgx/`
