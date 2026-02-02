package main

import (
"context"
"crypto/ecdsa"
"fmt"
"os"

"github.com/ethereum/go-ethereum"
"github.com/ethereum/go-ethereum/common"
"github.com/ethereum/go-ethereum/crypto"
"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
fmt.Println("=== X Chain 完整功能测试 ===")

if len(os.Args) < 2 {
fmt.Println("用法: go run full_test.go <RPC_URL>")
os.Exit(1)
}

ctx := context.Background()
client, err := ethclient.Dial(os.Args[1])
if err != nil {
fmt.Printf("❌ 连接失败: %v\n", err)
os.Exit(1)
}
defer client.Close()

privateKey, _ := crypto.GenerateKey()
publicKeyECDSA := privateKey.Public().(*ecdsa.PublicKey)
address := crypto.PubkeyToAddress(*publicKeyECDSA)

// 测试网络
chainID, _ := client.ChainID(ctx)
fmt.Printf("✓ 链 ID: %s\n", chainID)

// 测试预编译合约
fmt.Println("\n【预编译合约测试】")
testPrecompiled(ctx, client, address, "0x8000", "SGX_KEY_CREATE")
testPrecompiled(ctx, client, address, "0x8001", "SGX_KEY_GET_PUBLIC")
testPrecompiled(ctx, client, address, "0x8002", "SGX_SIGN")
testPrecompiled(ctx, client, address, "0x8003", "SGX_VERIFY")
testPrecompiled(ctx, client, address, "0x8004", "SGX_ECDH")
testPrecompiled(ctx, client, address, "0x8005", "SGX_RANDOM")

fmt.Println("\n✓ 测试完成")
}

func testPrecompiled(ctx context.Context, client *ethclient.Client, from common.Address, addr, name string) {
to := common.HexToAddress(addr)
data := make([]byte, 32)
msg := ethereum.CallMsg{From: from, To: &to, Data: data}
result, err := client.CallContract(ctx, msg, nil)
if err != nil {
fmt.Printf("  %s: ❌ %v\n", name, err)
} else {
fmt.Printf("  %s: ✓\n", name)
}
_ = result
}
