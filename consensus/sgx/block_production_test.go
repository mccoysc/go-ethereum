package sgx

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
	"github.com/ethereum/go-ethereum/params"
)

// TestBlockProductionBasic tests basic block production functionality
func TestBlockProductionBasic(t *testing.T) {
	// Setup test environment
	os.Setenv("SGX_TEST_MODE", "true")
	os.Setenv("GRAMINE_VERSION", "test")
	defer func() {
		os.Unsetenv("SGX_TEST_MODE")
		os.Unsetenv("GRAMINE_VERSION")
	}()

	// Create test blockchain
	db := rawdb.NewMemoryDatabase()
	gspec := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: types.GenesisAlloc{
			common.HexToAddress("0x1000"): {Balance: big.NewInt(1000000000000000000)},
		},
	}
	chain, _ := core.NewBlockChain(db, gspec, NewTestEngine(), &core.BlockChainConfig{})
	defer chain.Stop()

	// Create test transaction pool
	txpool := newMockTxPool()

	// Add some test transactions
	key, _ := crypto.GenerateKey()
	signer := types.LatestSigner(params.TestChainConfig)
	
	tx1 := types.NewTransaction(0, common.HexToAddress("0x2000"), big.NewInt(1000), 21000, big.NewInt(1000000000), nil)
	signedTx1, _ := types.SignTx(tx1, signer, key)
	
	tx2 := types.NewTransaction(1, common.HexToAddress("0x3000"), big.NewInt(2000), 21000, big.NewInt(1000000000), nil)
	signedTx2, _ := types.SignTx(tx2, signer, key)
	
	txpool.AddTx(signedTx1)
	txpool.AddTx(signedTx2)

	// Create engine and block producer
	config := DefaultConfig()
	attestor, verifier := createTestAttestorVerifier(t)
	engine := New(config, attestor, verifier)
	
	producer := NewBlockProducer(config, engine, txpool, chain)

	// Test block production
	t.Run("ProduceBlockNow", func(t *testing.T) {
		parent := chain.CurrentBlock()
		coinbase := common.HexToAddress("0x4000")
		
		txs := []*types.Transaction{signedTx1, signedTx2}
		
		block, err := producer.ProduceBlockNow(parent, txs, coinbase)
		if err != nil {
			t.Fatalf("Failed to produce block: %v", err)
		}
		
		if block == nil {
			t.Fatal("Produced block is nil")
		}
		
		if block.NumberU64() != parent.Number.Uint64()+1 {
			t.Errorf("Block number mismatch: got %d, want %d", block.NumberU64(), parent.Number.Uint64()+1)
		}
		
		if len(block.Transactions()) != 2 {
			t.Errorf("Transaction count mismatch: got %d, want 2", len(block.Transactions()))
		}
		
		t.Logf("Successfully produced block #%d with %d transactions", block.NumberU64(), len(block.Transactions()))
	})

	// Test automatic block production
	t.Run("AutomaticProduction", func(t *testing.T) {
		// Force old last block time to trigger production
		producer.SetLastBlockTime(time.Now().Add(-10 * time.Second))
		
		initialHeight := chain.CurrentBlock().Number.Uint64()
		
		// Start block producer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := producer.Start(ctx); err != nil {
			t.Fatalf("Failed to start block producer: %v", err)
		}
		defer producer.Stop()
		
		// Wait for block to be produced
		time.Sleep(2 * time.Second)
		
		currentHeight := chain.CurrentBlock().Number.Uint64()
		
		if currentHeight <= initialHeight {
			t.Errorf("No block produced: height stayed at %d", currentHeight)
		} else {
			t.Logf("Block produced: height increased from %d to %d", initialHeight, currentHeight)
		}
	})
}

// mockTxPool implements TxPool interface for testing
type mockTxPool struct {
	txs map[common.Address]types.Transactions
}

func newMockTxPool() *mockTxPool {
	return &mockTxPool{
		txs: make(map[common.Address]types.Transactions),
	}
}

func (p *mockTxPool) AddTx(tx *types.Transaction) {
	from, _ := types.Sender(types.LatestSigner(params.TestChainConfig), tx)
	p.txs[from] = append(p.txs[from], tx)
}

func (p *mockTxPool) Add(txs []*types.Transaction, sync bool) []error {
	for _, tx := range txs {
		p.AddTx(tx)
	}
	return nil
}

func (p *mockTxPool) Remove(txHash common.Hash) {
	// Simple implementation for testing
}

func (p *mockTxPool) Pending(enforceTips bool) map[common.Address][]*types.Transaction {
	result := make(map[common.Address][]*types.Transaction)
	for addr, txs := range p.txs {
		result[addr] = txs
	}
	return result
}

func (p *mockTxPool) PendingCount() int {
	count := 0
	for _, txs := range p.txs {
		count += len(txs)
	}
	return count
}

// NewTestEngine creates a test SGX engine
func NewTestEngine() *SGXEngine {
	config := DefaultConfig()
	
	// Create mock attestor and verifier
	attestor := &mockAttestor{}
	verifier := &mockVerifier{}
	
	return New(config, attestor, verifier)
}

// mockAttestor for testing
type mockAttestor struct{}

func (m *mockAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	return make([]byte, 64), nil
}

func (m *mockAttestor) SignInEnclave(data []byte) ([]byte, error) {
	return make([]byte, 65), nil
}

func (m *mockAttestor) GetProducerID() ([]byte, error) {
	return common.HexToAddress("0x1234").Bytes(), nil
}

// mockVerifier for testing
type mockVerifier struct{}

func (m *mockVerifier) VerifyQuote(quote []byte) error {
	return nil
}

func (m *mockVerifier) VerifySignature(data, signature, producerID []byte) error {
	return nil
}

func (m *mockVerifier) ExtractMREnclave(quote []byte) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *mockVerifier) ExtractInstanceID(quote []byte) ([]byte, error) {
	return make([]byte, 16), nil
}

func (m *mockVerifier) ExtractQuoteUserData(quote []byte) ([]byte, error) {
	return make([]byte, 64), nil
}

func (m *mockVerifier) ExtractPublicKeyFromQuote(quote []byte) ([]byte, error) {
	return make([]byte, 65), nil
}

func (m *mockVerifier) ExtractProducerID(quote []byte) ([]byte, error) {
	return common.HexToAddress("0x1234").Bytes(), nil
}

func (m *mockVerifier) VerifyQuoteComplete(input []byte, options map[string]interface{}) (*internalsgx.QuoteVerificationResult, error) {
	return &internalsgx.QuoteVerificationResult{
		Verified:  true,
		TCBStatus: "UpToDate",
		Measurements: internalsgx.QuoteMeasurements{
			MrEnclave: make([]byte, 32),
			MrSigner:  make([]byte, 32),
		},
	}, nil
}

