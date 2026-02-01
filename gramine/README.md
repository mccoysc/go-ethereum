# X Chain Gramine Integration

This directory contains the Gramine integration for X Chain, enabling the node to run in Intel SGX enclaves.

## Overview

The Gramine integration module (Module 07) is the complete integration solution that brings together all modules (01-06) and runs the entire Geth node in a Gramine SGX environment, forming a complete X Chain node.

## Architecture

```
X Chain Node (Docker Container)
│
├── Gramine Runtime
│   └── SGX Enclave
│       │
│       ├── Modified Geth
│       │   ├── Module 01: SGX Attestation (RA-TLS)
│       │   ├── Module 02: Consensus Engine (PoA-SGX)
│       │   ├── Module 03: Incentive Mechanism
│       │   ├── Module 04: Precompiled Contracts (Key Management)
│       │   ├── Module 05: Governance
│       │   └── Module 06: Data Storage
│       │
│       └── Encrypted Partitions
│           ├── /data/encrypted (Private keys)
│           ├── /data/secrets (Derived secrets)
│           └── /app/wallet (Blockchain data)
```

## Quick Start

### Prerequisites

- Docker installed
- (For SGX mode) SGX-capable CPU with driver installed
- (For SGX mode) AESM service running

### Method 1: Using Docker (Recommended for Production)

```bash
# Build the Docker image
./build-docker.sh v1.0.0

# Start the node
docker-compose up -d

# Verify node status
./verify-node-status.sh
```

### Method 2: Local Development (Fast Iteration)

```bash
# Compile geth in Gramine environment
./build-in-gramine.sh

# Option A: Local integration test (fastest)
./run-local.sh

# Option B: Gramine direct mode (simulation)
./rebuild-manifest.sh dev
./run-dev.sh direct

# Option C: Gramine SGX mode (requires hardware)
./rebuild-manifest.sh dev
./run-dev.sh sgx
```

## Scripts

### Build Scripts

- **`build-in-gramine.sh`** - Compile geth in Gramine Docker environment
- **`build-docker.sh [version] [mode]`** - Build complete Docker image
- **`rebuild-manifest.sh [dev|prod]`** - Quickly regenerate Gramine manifest
- **`setup-signing-key.sh`** - Generate/manage manifest signing key

### Run Scripts

- **`run-local.sh`** - Run in Gramine container (direct geth, no gramine wrapper)
- **`run-dev.sh [direct|sgx]`** - Run with gramine-direct or gramine-sgx
- **`start-xchain.sh`** - Container entrypoint script

### Validation Scripts

- **`check-sgx.sh`** - Check SGX hardware and driver support
- **`verify-node-status.sh`** - Verify running node status
- **`validate-integration.sh`** - Validate all module integration

### Deployment Scripts

- **`push-docker.sh [version]`** - Push Docker image to registry

## Development Workflow

### Initial Setup

```bash
# 1. Generate signing key
./setup-signing-key.sh

# 2. Compile geth
./build-in-gramine.sh

# 3. Generate manifest
./rebuild-manifest.sh dev

# 4. Test locally
./run-local.sh
```

### Daily Development Iteration

```bash
# 1. Modify code
vim ../consensus/sgx/consensus.go

# 2. Recompile (30 seconds)
./build-in-gramine.sh

# 3. Quick test (seconds)
./run-local.sh

# Or test with Gramine
./rebuild-manifest.sh dev
./run-dev.sh direct
```

### Prepare for Production

```bash
# Switch to production mode
./rebuild-manifest.sh prod

# Test in SGX
./run-dev.sh sgx

# Build Docker image
./build-docker.sh v1.0.0 prod

# Push to registry
./push-docker.sh v1.0.0
```

## Running Modes

### 1. Local Test Mode (`run-local.sh`)

**Purpose**: Fast integration testing without Gramine overhead

**Characteristics**:
- Runs geth directly in Gramine Docker container
- No gramine-sgx/gramine-direct wrapper
- SGX functions use mock data
- Fastest startup (seconds)

**Use Cases**:
- Feature development
- Integration testing
- Quick functionality validation

### 2. Gramine Direct Mode (`run-dev.sh direct`)

**Purpose**: Test Gramine integration without SGX hardware

**Characteristics**:
- Runs geth through gramine-direct
- Simulates SGX environment
- No real SGX security guarantees
- No SGX hardware required

**Use Cases**:
- Gramine integration testing
- Development on non-SGX hardware
- CI/CD testing

### 3. Gramine SGX Mode (`run-dev.sh sgx`)

**Purpose**: Run in real SGX enclave

**Characteristics**:
- Runs geth through gramine-sgx
- Full SGX hardware protection
- Requires SGX-capable CPU
- Real remote attestation

**Use Cases**:
- Security testing
- Performance benchmarking
- Pre-production validation

### 4. Docker Production Mode

**Purpose**: Production deployment

**Characteristics**:
- Complete Docker container
- Gramine + SGX environment
- MRENCLAVE sealing
- Remote attestation enabled

**Use Cases**:
- Production deployment
- Network validators
- Public nodes

## Configuration

### Manifest Parameters (Fixed, Affects MRENCLAVE)

Set in `geth.manifest.template`, these cannot be changed at runtime:

- `XCHAIN_ENCRYPTED_PATH`: Path to encrypted partition
- `XCHAIN_SECRET_PATH`: Path to secrets storage
- `XCHAIN_GOVERNANCE_CONTRACT`: Governance contract address
- `XCHAIN_SECURITY_CONFIG_CONTRACT`: Security config contract address

### Runtime Parameters (Environment Variables)

Can be set when running the container:

- `XCHAIN_NETWORK_ID`: Network ID (default: 762385986)
- `XCHAIN_DATA_DIR`: Data directory path
- `XCHAIN_RPC_PORT`: RPC port (default: 8545)
- `XCHAIN_WS_PORT`: WebSocket port (default: 8546)
- `XCHAIN_P2P_PORT`: P2P port (default: 30303)

## Files

### Configuration Files
- `geth.manifest.template` - Gramine manifest template
- `genesis-local.json` - Local test genesis configuration
- `docker-compose.yml` - Docker Compose configuration

### Scripts (see above sections)

### Generated Files (not committed)
- `geth.manifest` - Generated manifest
- `geth.manifest.sgx` - Signed manifest
- `MRENCLAVE.txt` - MRENCLAVE measurement value
- `enclave-key.pem` - Manifest signing key

## Testing

### Run Integration Tests

```bash
# Run all integration tests
go test -tags=integration ./gramine/...

# Run specific test
go test -tags=integration -run TestSGXHardwareDetection ./gramine/
```

### Manual Validation

```bash
# Check SGX hardware
./check-sgx.sh

# Verify node is running
./verify-node-status.sh

# Validate all modules
./validate-integration.sh
```

## Troubleshooting

### SGX Device Not Found

**Error**: `/dev/sgx_enclave` not found

**Solutions**:
1. Verify CPU supports SGX: `cpuid | grep SGX`
2. Enable SGX in BIOS
3. Install SGX driver
4. Use `run-dev.sh direct` for development without SGX

### AESM Service Not Running

**Error**: AESM socket not found

**Solutions**:
```bash
# Start AESM service
sudo systemctl start aesmd

# Or manually
sudo /opt/intel/sgx-aesm-service/aesm/aesm_service &
```

### Permission Denied

**Solution**: Some operations require sudo:
```bash
sudo ./run-dev.sh sgx
```

### Gramine Command Not Found

**Solution**: Install Gramine:
```bash
# Ubuntu/Debian
sudo apt install gramine

# Or use Docker (recommended)
./build-docker.sh
```

## Performance

### Build/Test Time Comparison

| Operation | Traditional Docker | New Quick Method | Time Saved |
|-----------|-------------------|------------------|------------|
| Recompile geth | 30s | 30s | - |
| Build Docker | 5-10 min | - | - |
| Generate manifest | - | 5s | - |
| Start test | 30s | 5s (direct) | 83% |
| **Total** | **6-11 min** | **40s** | **93%** |

## Security Considerations

### Development Mode (MRSIGNER sealing)
- ✅ Fast iteration (no data migration needed)
- ⚠️ Lower security (same signer can access data)
- Use for development and testing only

### Production Mode (MRENCLAVE sealing)
- ✅ Highest security (code-bound sealing)
- ⚠️ Data migration needed on code changes
- Required for production deployment

## Documentation

For detailed information, see:
- [Module 07 Documentation](../docs/modules/07-gramine-integration.md)
- [Architecture Overview](../ARCHITECTURE.md)
- [SGX Attestation (Module 01)](../docs/modules/01-sgx-attestation.md)

## Support

For issues and questions:
- Check existing documentation
- Review troubleshooting section
- Check GitHub issues
- Contact the development team
