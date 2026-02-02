# X Chain Deployment Quick Start Guide

This guide provides quick start instructions for deploying X Chain nodes using the Gramine integration.

## Prerequisites

- Docker installed and running
- (Optional for SGX mode) SGX-capable CPU with SGX enabled in BIOS
- (Optional for SGX mode) SGX driver and AESM service installed

## Quick Start: Development Testing

### 1. Build and Test Locally (Fastest)

```bash
# Compile geth in Gramine environment
cd gramine
./build-in-gramine.sh

# Run local integration test (no Gramine wrapper)
./run-local.sh
```

This mode:
- ✅ Fastest startup (seconds)
- ✅ No SGX hardware required
- ✅ Perfect for feature development
- ⚠️ Uses mock SGX data

### 2. Test with Gramine Direct (Simulation)

```bash
# Build geth
cd gramine
./build-in-gramine.sh

# Generate manifest in dev mode
./rebuild-manifest.sh dev

# Run with gramine-direct
./run-dev.sh direct
```

This mode:
- ✅ Tests Gramine integration
- ✅ No SGX hardware required
- ✅ Good for integration testing
- ⚠️ No real SGX security

### 3. Test with SGX (Requires Hardware)

```bash
# Check SGX hardware
cd gramine
./check-sgx.sh

# Build and run
./build-in-gramine.sh
./rebuild-manifest.sh dev
./run-dev.sh sgx
```

This mode:
- ✅ Real SGX enclave
- ✅ Full security features
- ✅ Remote attestation enabled
- ⚠️ Requires SGX hardware

## Production Deployment

### Using Docker Compose (Recommended)

```bash
# 1. Build the Docker image
cd gramine
./build-docker.sh v1.0.0 prod

# 2. Create data directories
mkdir -p data/{encrypted,secrets,wallet} logs

# 3. Start the node
cd ..
docker-compose up -d

# 4. Verify node status
./gramine/verify-node-status.sh

# 5. Validate all modules
./gramine/validate-integration.sh
```

### Verify Deployment

After starting the node, verify it's working:

```bash
# Check node is running
docker ps | grep xchain-node

# Check logs
docker logs xchain-node

# Verify RPC endpoint
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Run comprehensive validation
./gramine/verify-node-status.sh
```

## Configuration

### Network Configuration

Set via environment variables in `docker-compose.yml`:

```yaml
environment:
  - XCHAIN_NETWORK_ID=762385986
  - XCHAIN_RPC_PORT=8545
  - XCHAIN_WS_PORT=8546
  - XCHAIN_P2P_PORT=30303
```

### Fixed Security Parameters

These are set during Docker build and affect MRENCLAVE:

```dockerfile
ARG GOVERNANCE_CONTRACT=0x0000000000000000000000000000000000001001
ARG SECURITY_CONFIG_CONTRACT=0x0000000000000000000000000000000000001002
```

## Monitoring

### Health Check

```bash
# Docker health status
docker ps --filter name=xchain-node --format "{{.Status}}"

# Manual health check
curl -f http://localhost:8545 \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### View Logs

```bash
# Follow logs
docker logs -f xchain-node

# Last 100 lines
docker logs --tail 100 xchain-node
```

### Verify MRENCLAVE

```bash
# Get MRENCLAVE from running container
docker exec xchain-node cat /app/MRENCLAVE.txt

# Or from local build
cat gramine/MRENCLAVE.txt
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs xchain-node

# Check SGX devices are mapped
docker inspect xchain-node | grep -A5 Devices
```

### SGX Device Not Found

If you see "SGX device not found":

1. Check if you need SGX: Development can use `gramine-direct`
2. Verify CPU supports SGX: `cpuid | grep SGX`
3. Enable SGX in BIOS
4. Install SGX driver: Check `/dev/sgx_enclave`

### AESM Service Issues

```bash
# Check if AESM is running
pgrep aesm_service

# Start AESM
sudo systemctl start aesmd

# Check socket
ls -la /var/run/aesmd/aesm.socket
```

### RPC Not Responding

```bash
# Check if port is listening
netstat -tlnp | grep 8545

# Check firewall
sudo ufw status

# Test from inside container
docker exec xchain-node curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

## Upgrading

### With MRENCLAVE Sealing (Production)

```bash
# 1. Backup data
tar -czf xchain-backup-$(date +%Y%m%d).tar.gz data/

# 2. Build new version
./gramine/build-docker.sh v1.1.0 prod

# 3. Stop old node
docker-compose down

# 4. Update docker-compose.yml with new version
# 5. Start new node
docker-compose up -d

# Note: Data migration may be required due to MRENCLAVE change
```

### With MRSIGNER Sealing (Development)

```bash
# 1. Build new version
./gramine/build-docker.sh v1.1.0-dev dev

# 2. Restart
docker-compose down
docker-compose up -d

# Data is automatically accessible (same signing key)
```

## Security Checklist

Before production deployment:

- [ ] SGX hardware verified (`./gramine/check-sgx.sh`)
- [ ] Using MRENCLAVE sealing (not MRSIGNER)
- [ ] Debug mode disabled in manifest
- [ ] Contract addresses are correct
- [ ] MRENCLAVE recorded for governance whitelist
- [ ] Backup encryption key secured
- [ ] Network ports properly configured
- [ ] Firewall rules applied
- [ ] Monitoring alerts configured

## Getting Help

1. Check logs: `docker logs xchain-node`
2. Run validation: `./gramine/verify-node-status.sh`
3. Check documentation: `./gramine/README.md`
4. Review module docs: `./docs/modules/07-gramine-integration.md`

## Development Workflow Summary

```bash
# Quick iteration cycle (40 seconds)
vim consensus/sgx/consensus.go     # Make changes
./gramine/build-in-gramine.sh      # Rebuild (30s)
./gramine/run-local.sh             # Test (5s)

# vs Traditional Docker (6-11 minutes)
vim consensus/sgx/consensus.go
make geth                           # 30s
docker build ...                    # 5-10min
docker run ...                      # 30s
```

## Next Steps

1. **For Development**: Use `./gramine/run-local.sh` for fast iteration
2. **For Integration Testing**: Use `./gramine/run-dev.sh direct`
3. **For Security Testing**: Use `./gramine/run-dev.sh sgx`
4. **For Production**: Use Docker Compose with full validation

## References

- [Module 07 Full Documentation](../docs/modules/07-gramine-integration.md)
- [X Chain Architecture](../ARCHITECTURE.md)
- [Gramine Documentation](https://gramine.readthedocs.io)
- [SGX Developer Guide](https://www.intel.com/content/www/us/en/developer/tools/software-guard-extensions/overview.html)
