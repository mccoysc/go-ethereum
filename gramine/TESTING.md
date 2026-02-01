# X Chain Gramine Integration Testing Guide

This document describes how to test the Gramine integration (Module 07) and validate that all modules work correctly together.

## Testing Layers

The X Chain testing strategy uses multiple layers:

```
Layer 1: Local Integration Test (fastest)
   └─> Layer 2: Gramine Direct Mode (simulation)
       └─> Layer 3: Gramine SGX Mode (real hardware)
           └─> Layer 4: Docker Production Mode
```

Each layer builds on the previous one, providing progressively more realistic environments.

## Layer 1: Local Integration Test

**Purpose**: Fast feature development and integration testing

**Setup**:
```bash
cd gramine
./build-in-gramine.sh
```

**Run**:
```bash
./run-local.sh
```

**What it tests**:
- ✅ Geth compiles and runs
- ✅ All modules load correctly
- ✅ RPC endpoints work
- ✅ Basic blockchain functionality
- ⚠️ SGX functions use mock data

**Expected output**:
```
=== X Chain 本地集成测试（Gramine 容器环境）===
✓ 找到 geth: /path/to/geth
✓ 测试数据目录: /path/to/test-data
...
INFO [02-01|09:00:00.000] Starting Geth...
```

**Validation**:
```bash
# In another terminal
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

## Layer 2: Gramine Direct Mode

**Purpose**: Test Gramine integration without SGX hardware

**Setup**:
```bash
cd gramine
./build-in-gramine.sh
./rebuild-manifest.sh dev
```

**Run**:
```bash
./run-dev.sh direct
```

**What it tests**:
- ✅ Gramine manifest configuration
- ✅ Encrypted partition simulation
- ✅ Manifest parameter loading
- ✅ File system mounting
- ⚠️ No real SGX security

**Expected output**:
```
=== X Chain 节点快速启动（开发模式）===
运行配置:
  模式: direct
  Network ID: 762385986
使用 gramine-direct 模拟模式运行
```

**Validation**:
Run the same RPC calls as Layer 1.

## Layer 3: Gramine SGX Mode

**Purpose**: Test in real SGX enclave

**Prerequisites**:
```bash
# Check SGX hardware
./check-sgx.sh
```

**Setup**:
```bash
./rebuild-manifest.sh dev  # or prod for production mode
```

**Run**:
```bash
./run-dev.sh sgx
```

**What it tests**:
- ✅ Real SGX enclave execution
- ✅ MRENCLAVE calculation
- ✅ Encrypted partition with SGX sealing
- ✅ Remote attestation (if enabled)
- ✅ Full security guarantees

**Expected output**:
```
=== X Chain 节点快速启动（开发模式）===
使用 gramine-sgx SGX 模式运行
说明: 此模式在真实 SGX enclave 中运行
```

**Validation**:
```bash
# Verify running in enclave
ps aux | grep gramine-sgx

# Check MRENCLAVE
cat gramine/MRENCLAVE.txt
```

## Layer 4: Docker Production Mode

**Purpose**: Full production deployment testing

**Setup**:
```bash
./build-docker.sh v1.0.0 prod
```

**Run**:
```bash
cd ..
docker-compose up -d
```

**What it tests**:
- ✅ Complete Docker deployment
- ✅ All modules integrated
- ✅ Production configuration
- ✅ Volume persistence
- ✅ Network configuration

**Validation**:
```bash
# Comprehensive validation
./gramine/verify-node-status.sh
./gramine/validate-integration.sh
```

## Module-Specific Tests

### Module 01: SGX Attestation

```bash
# Test SGX hardware detection
go test -v ./internal/sgx -run TestSGXHardware

# Verify attestor works
go test -v ./internal/sgx -run TestAttestor
```

**Manual verification**:
```bash
# Check MRENCLAVE is generated
docker exec xchain-node cat /app/MRENCLAVE.txt
```

### Module 02: Consensus Engine

**Test latest block**:
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}'
```

**Expected**: Block with PoA-SGX extra data

### Module 03: Incentive Mechanism

**Check incentive contract**:
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "method":"eth_getCode",
    "params":["0x0000000000000000000000000000000000001003","latest"],
    "id":1
  }'
```

### Module 04: Precompiled Contracts

**Test key creation contract**:
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "method":"eth_call",
    "params":[{"to":"0x8000","data":"0x"},"latest"],
    "id":1
  }'
```

**Expected**: Contract responds (even if with error for invalid call)

### Module 05: Governance

**Verify governance contract address**:
```bash
docker exec xchain-node printenv XCHAIN_GOVERNANCE_CONTRACT
```

**Expected**: `0x0000000000000000000000000000000000001001`

### Module 06: Data Storage

**Test encrypted partition**:
```bash
# Check partitions are mounted
docker exec xchain-node ls -la /data/encrypted
docker exec xchain-node ls -la /data/secrets
docker exec xchain-node ls -la /app/wallet
```

**Expected**: All directories accessible

## Integration Tests

### Run Go Integration Tests

```bash
# Run all integration tests
go test -tags=integration -v ./gramine/...

# Run specific test
go test -tags=integration -v -run TestSGXHardwareDetection ./gramine/
```

### Manual Integration Validation

```bash
# Comprehensive validation script
./gramine/validate-integration.sh
```

**Expected output**:
```
=== X Chain Module Integration Validation ===
[01/06] Validating SGX Attestation Module...
  ✓ SGX attestation module is operational
[02/06] Validating Consensus Engine Module...
  ✓ PoA-SGX consensus engine is working
...
=== All Modules Validated ===
```

## Performance Testing

### Build Time Comparison

```bash
# Traditional method
time (make geth && docker build -t test -f Dockerfile.xchain .)

# New fast method
time (./gramine/build-in-gramine.sh && ./gramine/rebuild-manifest.sh dev)
```

**Expected**: New method ~93% faster for iteration

### Runtime Performance

```bash
# Measure block processing time
# Compare direct mode vs SGX mode
```

## Continuous Integration Tests

### CI Pipeline Test Suite

```bash
# 1. Lint check
go fmt ./internal/sgx/... ./internal/config/...
go vet ./internal/sgx/... ./internal/config/...

# 2. Unit tests
go test ./internal/sgx/...
go test ./internal/config/...

# 3. Build test
make geth

# 4. Integration tests (if SGX available)
go test -tags=integration ./gramine/...

# 5. Script syntax check
bash -n gramine/*.sh
```

## Troubleshooting Tests

### Test Failures in Layer 1

**Symptom**: `run-local.sh` fails to start

**Debug**:
```bash
# Check geth binary exists
ls -la build/bin/geth

# Check Docker is running
docker ps

# Run with verbose logging
XCHAIN_VERBOSITY=5 ./gramine/run-local.sh
```

### Test Failures in Layer 2

**Symptom**: `gramine-direct` fails

**Debug**:
```bash
# Check Gramine installation
which gramine-direct

# Check manifest
cat gramine/geth.manifest

# Run Gramine directly with debug
gramine-direct geth --help
```

### Test Failures in Layer 3

**Symptom**: `gramine-sgx` fails

**Debug**:
```bash
# Check SGX devices
ls -la /dev/sgx_*

# Check AESM service
systemctl status aesmd

# Check manifest signature
gramine-sgx-sigstruct-view gramine/geth.manifest.sgx
```

### Test Failures in Layer 4

**Symptom**: Docker container won't start

**Debug**:
```bash
# Check container logs
docker logs xchain-node

# Check volume mounts
docker inspect xchain-node | grep -A10 Mounts

# Check SGX device mapping
docker inspect xchain-node | grep -A5 Devices
```

## Test Checklist

Before considering Module 07 complete:

**Go Code**:
- [ ] All Go packages compile: `go build ./internal/sgx/... ./internal/config/...`
- [ ] Unit tests pass: `go test ./internal/sgx/... ./internal/config/...`
- [ ] Integration tests compile: `go test -tags=integration -c ./gramine/`
- [ ] Code formatted: `go fmt ./...`
- [ ] No vet warnings: `go vet ./...`

**Scripts**:
- [ ] All scripts have correct syntax: `bash -n gramine/*.sh`
- [ ] Scripts are executable: `ls -la gramine/*.sh`
- [ ] Scripts run without errors (where applicable)

**Build**:
- [ ] Geth compiles: `make geth`
- [ ] Docker image builds: `./gramine/build-docker.sh test`
- [ ] Manifest generates: `./gramine/rebuild-manifest.sh dev`

**Runtime**:
- [ ] Layer 1 runs: `./gramine/run-local.sh`
- [ ] Layer 2 runs: `./gramine/run-dev.sh direct`
- [ ] Layer 3 runs (if SGX available): `./gramine/run-dev.sh sgx`
- [ ] Layer 4 runs: `docker-compose up`

**Validation**:
- [ ] Node status verifies: `./gramine/verify-node-status.sh`
- [ ] Integration validates: `./gramine/validate-integration.sh`
- [ ] All modules accessible
- [ ] RPC endpoints respond

**Documentation**:
- [ ] README complete
- [ ] Deployment guide complete
- [ ] All scripts documented
- [ ] Architecture matches implementation

## Success Criteria

Module 07 is successfully implemented when:

1. ✅ All Go code compiles without errors
2. ✅ All tests pass (unit + integration)
3. ✅ All 4 test layers work correctly
4. ✅ All 6 modules validate successfully
5. ✅ Docker deployment works end-to-end
6. ✅ MRENCLAVE is generated correctly
7. ✅ Encrypted partitions mount properly
8. ✅ Parameter validation works
9. ✅ Documentation is complete
10. ✅ Matches ARCHITECTURE.md requirements

## Next Steps After Testing

1. **Security Audit**: Review MRENCLAVE sealing strategy
2. **Performance Tuning**: Optimize enclave size and thread count
3. **Monitoring Setup**: Add metrics and alerting
4. **Production Hardening**: Review security checklist
5. **Documentation**: Update with production learnings

## References

- [Module 07 Documentation](../docs/modules/07-gramine-integration.md)
- [Gramine Testing Guide](https://gramine.readthedocs.io/en/latest/testing.html)
- [SGX SDK Documentation](https://download.01.org/intel-sgx/latest/linux-latest/docs/)
