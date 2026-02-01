#!/bin/bash
set -e

# 读取编译后的合约字节码
SECURITY_CODE=$(cat build/SecurityConfigContract.bin)
GOVERNANCE_CODE=$(cat build/GovernanceContract.bin)
INCENTIVE_CODE=$(cat build/IncentiveContract.bin)

# 生成创世配置
cat > ../test/integration/genesis-complete.json << EOF
{
  "config": {
    "chainId": 762385986,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "terminalTotalDifficulty": 0,
    "terminalTotalDifficultyPassed": true,
    "sgx": {
      "period": 5,
      "epoch": 30000,
      "governanceContract": "0x0000000000000000000000000000000000001001",
      "securityConfig": "0x0000000000000000000000000000000000001002",
      "incentiveContract": "0x0000000000000000000000000000000000001003"
    }
  },
  "nonce": "0x0",
  "timestamp": "0x0",
  "extraData": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "gasLimit": "0x47b760",
  "difficulty": "0x1",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "alloc": {
    "0x0000000000000000000000000000000000008000": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b506004361060285760003560e01c8063a87d942c14602d575b600080fd5b60336047565b604051603e9190605b565b60405180910390f35b60008060405160200160509190607e565b6040516020818303038152906040529050919050565b600081519050919050565b6000819050919050565b6000607282605a565b915060788260655656"
    },
    "0x0000000000000000000000000000000000008001": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008002": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008003": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008004": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008005": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008006": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008007": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000008008": {
      "balance": "0x0",
      "code": "0x6080604052348015600f57600080fd5b50"
    },
    "0x0000000000000000000000000000000000001001": {
      "balance": "0x0",
      "code": "0x${GOVERNANCE_CODE}"
    },
    "0x0000000000000000000000000000000000001002": {
      "balance": "0x0",
      "code": "0x${SECURITY_CODE}"
    },
    "0x0000000000000000000000000000000000001003": {
      "balance": "0x0",
      "code": "0x${INCENTIVE_CODE}"
    },
    "0xa875022f57343979503b4a95637315064eb01698": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000000"
    }
  },
  "number": "0x0",
  "gasUsed": "0x0",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "baseFeePerGas": null
}
EOF

echo "创世配置已生成: test/integration/genesis-complete.json"
ls -lh ../test/integration/genesis-complete.json
