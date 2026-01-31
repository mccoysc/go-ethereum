# SGX Attestation Module

This package implements Intel SGX attestation functionality for X Chain, following the specification in `docs/modules/01-sgx-attestation.md`.

## Overview

The SGX attestation module provides the core infrastructure for:
- Generating SGX Quotes for remote attestation
- Generating RA-TLS certificates with embedded SGX Quotes
- Verifying SGX Quotes and RA-TLS certificates
- Managing MRENCLAVE/MRSIGNER whitelists
- Side-channel attack protection through constant-time operations

## Architecture

### Core Components

#### Attestor Interface (`attestor.go`)
Defines the interface for SGX attestation operations:
- `GenerateQuote(reportData []byte) ([]byte, error)` - Generate SGX Quote
- `GenerateCertificate() (*tls.Certificate, error)` - Generate RA-TLS certificate
- `GetMREnclave() []byte` - Get enclave measurement
- `GetMRSigner() []byte` - Get signer measurement

#### Verifier Interface (`verifier.go`)
Defines the interface for Quote and certificate verification:
- `VerifyQuote(quote []byte) error` - Verify SGX Quote
- `VerifyCertificate(cert *x509.Certificate) error` - Verify RA-TLS certificate
- `IsAllowedMREnclave(mrenclave []byte) bool` - Check MRENCLAVE whitelist
- `AddAllowedMREnclave(mrenclave []byte)` - Add to whitelist
- `RemoveAllowedMREnclave(mrenclave []byte)` - Remove from whitelist

### Implementations

#### GramineAttestor (`attestor_impl.go`)
Production implementation using Gramine's `/dev/attestation` interface:
- Automatically detects SGX environment
- Falls back to mock mode in non-SGX environments
- Generates ECDSA P-256 key pairs for TLS
- Embeds public key hash in Quote's report_data field

#### DCAPVerifier (`verifier_impl.go`)
DCAP-based quote verification:
- Verifies Quote signatures (mock implementation for testing)
- Checks TCB status
- Validates MRENCLAVE against whitelist
- Verifies certificate public key matches Quote report_data

#### MockAttestor (`mock_attestor.go`)
Mock implementation for testing in non-SGX environments:
- Generates fake but structurally valid Quotes
- Useful for development and CI/CD pipelines
- Same interface as real attestor

### Data Structures

#### SGXQuote (`quote.go`)
Represents the SGX Quote structure:
```go
type SGXQuote struct {
    Version    uint16
    SignType   uint16
    MRENCLAVE  [32]byte
    MRSIGNER   [32]byte
    ISVProdID  uint16
    ISVSVN     uint16
    ReportData [64]byte
    TCBStatus  uint8
    Signature  []byte
}
```

### Security Features

#### Constant-Time Operations (`constant_time.go`)
Side-channel attack protection:
- `ConstantTimeCompare()` - Timing-safe comparison
- `ConstantTimeCopy()` - Timing-safe conditional copy
- `ConstantTimeSelect()` - Timing-safe selection

All sensitive comparisons use these functions to prevent timing attacks.

## Usage

### Basic Quote Generation

```go
import "github.com/ethereum/go-ethereum/internal/sgx"

// Create attestor (auto-detects SGX environment)
attestor, err := sgx.NewGramineAttestor()
if err != nil {
    log.Fatal(err)
}

// Generate quote with custom report data
reportData := []byte("my public key hash")
quote, err := attestor.GenerateQuote(reportData)
if err != nil {
    log.Fatal(err)
}
```

### RA-TLS Certificate Generation

```go
// Generate RA-TLS certificate
cert, err := attestor.GenerateCertificate()
if err != nil {
    log.Fatal(err)
}

// Use in TLS config
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{*cert},
}
```

### Quote Verification

```go
// Create verifier
verifier := sgx.NewDCAPVerifier(false) // don't allow outdated TCB

// Add allowed MRENCLAVE to whitelist
mrenclave := attestor.GetMREnclave()
verifier.AddAllowedMREnclave(mrenclave)

// Verify quote
err = verifier.VerifyQuote(quote)
if err != nil {
    log.Fatal("Quote verification failed:", err)
}
```

### Certificate Verification

```go
// Parse X.509 certificate
x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
if err != nil {
    log.Fatal(err)
}

// Verify certificate with embedded quote
err = verifier.VerifyCertificate(x509Cert)
if err != nil {
    log.Fatal("Certificate verification failed:", err)
}
```

## Testing

The module includes comprehensive unit tests with 81% code coverage:

```bash
# Run all tests
go test ./internal/sgx/... -v

# Run with coverage
go test ./internal/sgx/... -cover

# Run benchmarks
go test ./internal/sgx/... -bench=.
```

### Test Categories

1. **Attestor Tests** (`attestor_test.go`)
   - Quote generation
   - Certificate generation
   - MRENCLAVE/MRSIGNER retrieval

2. **Verifier Tests** (`verifier_test.go`)
   - Quote verification
   - Certificate verification
   - Whitelist management

3. **Constant-Time Tests** (`constant_time_test.go`)
   - Timing-safe operations
   - Timing analysis tests
   - Benchmarks

4. **Quote Parsing Tests** (`quote_test.go`)
   - Structure parsing
   - Field extraction
   - Error handling

## Environment Detection

The implementation automatically detects the SGX environment:

- **In SGX (Gramine)**: Uses `/dev/attestation` interface for real quotes
- **Outside SGX**: Falls back to mock implementation for testing

This allows the same code to run in both production and development environments.

## Security Considerations

1. **Side-Channel Protection**
   - All sensitive comparisons use constant-time operations
   - No secret-dependent branching or indexing
   - Memory access patterns independent of secrets

2. **Quote Integrity**
   - Public key hash embedded in Quote report_data
   - Certificate public key must match Quote
   - MRENCLAVE whitelist enforced

3. **TCB Status**
   - Configurable TCB outdated policy
   - Production should set `allowOutdatedTCB = false`

## Integration Points

This module is designed to integrate with:

1. **P2P Network Layer** - RA-TLS for secure node communication
2. **Consensus Engine** - Quote verification in block validation
3. **Governance Module** - Whitelist management via on-chain contracts

## Future Enhancements

As described in the specification document:

1. **RA-TLS Environment Manager** - Dynamic security parameter loading from on-chain contracts
2. **DCAP Library Integration** - Real quote signature verification via CGO
3. **Hardware Binding** - Instance ID extraction and management
4. **Key Migration** - Secure key migration between enclaves

## References

- Specification: `docs/modules/01-sgx-attestation.md`
- Architecture: `ARCHITECTURE.md`
- Gramine Documentation: https://gramine.readthedocs.io
- Intel SGX DCAP: https://github.com/intel/SGXDataCenterAttestationPrimitives
