package sgx

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
	"github.com/ethereum/go-ethereum/trie"
)

// createTestAttestorVerifier creates real Module 01 attestor and verifier for testing
// These will use mock implementations when not in SGX environment
func createTestAttestorVerifier(t *testing.T) (Attestor, Verifier) {
	// Set test mode to avoid SGX requirements
	os.Setenv("SGX_TEST_MODE", "true")
	
	// Use Module 01's real implementations which auto-detect SGX environment
	// and fall back to mock mode if not available
	m01Attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		t.Fatalf("Failed to create Module 01 attestor: %v", err)
	}

	m01Verifier := internalsgx.NewDCAPVerifier(true) // Use true for testenv

	// Wrap in adapters that add consensus-specific methods
	attestor := newTestAttestorAdapter(m01Attestor)
	verifier := newTestVerifierAdapter(m01Verifier)

	return attestor, verifier
}

// testAttestorAdapter wraps Module 01 Attestor and adds consensus-specific methods
type testAttestorAdapter struct {
	internalsgx.Attestor
	privateKey *ecdsa.PrivateKey
}

func newTestAttestorAdapter(m01Attestor internalsgx.Attestor) *testAttestorAdapter {
	// Generate a private key for signing
	privateKey, _ := crypto.GenerateKey()
	return &testAttestorAdapter{
		Attestor:   m01Attestor,
		privateKey: privateKey,
	}
}

func (a *testAttestorAdapter) SignInEnclave(data []byte) ([]byte, error) {
	// In tests, use standard ECDSA signing
	hash := crypto.Keccak256Hash(data)
	return crypto.Sign(hash.Bytes(), a.privateKey)
}

func (a *testAttestorAdapter) GetProducerID() ([]byte, error) {
	// Return address derived from public key
	address := crypto.PubkeyToAddress(a.privateKey.PublicKey)
	return address.Bytes(), nil
}

// testVerifierAdapter wraps Module 01 Verifier and adds consensus-specific methods
type testVerifierAdapter struct {
	verifier *internalsgx.DCAPVerifier
}

func newTestVerifierAdapter(m01Verifier internalsgx.Verifier) *testVerifierAdapter {
	// Type assert to get DCAPVerifier for additional methods
	dcapVerifier, ok := m01Verifier.(*internalsgx.DCAPVerifier)
	if !ok {
		panic("Expected DCAPVerifier")
	}
	return &testVerifierAdapter{
		verifier: dcapVerifier,
	}
}

func (v *testVerifierAdapter) VerifyQuote(quote []byte) error {
	return v.verifier.VerifyQuote(quote)
}

func (v *testVerifierAdapter) VerifyQuoteComplete(input []byte, options map[string]interface{}) (*internalsgx.QuoteVerificationResult, error) {
	return v.verifier.VerifyQuoteComplete(input, options)
}

func (v *testVerifierAdapter) ExtractInstanceID(quote []byte) ([]byte, error) {
	// Use DCAPVerifier's ExtractInstanceID method
	id, err := internalsgx.ExtractInstanceID(quote)
	if err != nil {
		return nil, err
	}
	return id.CPUInstanceID, nil
}

func (v *testVerifierAdapter) ExtractQuoteUserData(quote []byte) ([]byte, error) {
	return internalsgx.ExtractReportData(quote)
}

func (v *testVerifierAdapter) ExtractPublicKeyFromQuote(quote []byte) ([]byte, error) {
	// Extract report data which should contain public key
	reportData, err := internalsgx.ExtractReportData(quote)
	if err != nil {
		return nil, err
	}
	// First 65 bytes should be the public key in uncompressed format
	if len(reportData) < 65 {
		return nil, fmt.Errorf("report data too short for public key")
	}
	return reportData[:65], nil
}

func (v *testVerifierAdapter) ExtractProducerID(quote []byte) ([]byte, error) {
	// Use VerifyQuoteComplete to get instance ID, then derive producer ID
	result, err := v.verifier.VerifyQuoteComplete(quote, nil)
	if err != nil {
		return nil, err
	}
	// Producer ID is derived from instance ID (first 20 bytes or hash)
	if len(result.Measurements.PlatformInstanceID) >= 20 {
		return result.Measurements.PlatformInstanceID[:20], nil
	}
	return result.Measurements.PlatformInstanceID, nil
}

func (v *testVerifierAdapter) VerifySignature(data, signature, producerID []byte) error {
	if len(signature) != 65 {
		return ErrInvalidSignature
	}
	if len(producerID) != 20 {
		return ErrInvalidProducerID
	}

	hash := crypto.Keccak256Hash(data)
	pubKey, err := crypto.SigToPub(hash.Bytes(), signature)
	if err != nil {
		return err
	}

	recoveredAddress := crypto.PubkeyToAddress(*pubKey)
	expectedAddress := common.BytesToAddress(producerID)
	if recoveredAddress != expectedAddress {
		return ErrInvalidSignature
	}

	return nil
}

// Note: VerifyQuoteComplete, ExtractInstanceID, ExtractQuoteUserData, ExtractPublicKeyFromQuote, ExtractProducerID
// are inherited from the wrapped internal/sgx.Verifier and use real implementations
// (which will use real quote data in testenv mode with build tags)

// TestNewEngine tests engine creation
func TestNewEngine(t *testing.T) {
	// Setup test environment variables
	os.Setenv("SGX_TEST_MODE", "true")
	os.Setenv("GRAMINE_VERSION", "test")
	defer func() {
		os.Unsetenv("SGX_TEST_MODE")
		os.Unsetenv("GRAMINE_VERSION")
	}()
	
	config := DefaultConfig()
	attestor, verifier := createTestAttestorVerifier(t)

	engine := New(config, attestor, verifier)
	if engine == nil {
		t.Fatal("Failed to create SGX engine")
	}

	if engine.config == nil {
		t.Fatal("Engine config is nil")
	}
}

// TestBlockQualityScorer tests block quality scoring
func TestBlockQualityScorer(t *testing.T) {
	config := DefaultConfig().QualityConfig
	scorer := NewBlockQualityScorer(config)

	// Create a mock block with transactions
	header := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		GasUsed:    24000000, // 80% utilization
		Difficulty: big.NewInt(1),
	}

	txs := make([]*types.Transaction, 30)
	for i := 0; i < 30; i++ {
		tx := types.NewTransaction(
			uint64(i),
			common.HexToAddress("0x1234"),
			big.NewInt(1000),
			21000,
			big.NewInt(1000000000),
			nil,
		)
		txs[i] = tx
	}

	block := types.NewBlock(header, &types.Body{Transactions: txs}, nil, trie.NewStackTrie(nil))

	quality := scorer.CalculateQuality(block)

	if quality.TxCount != 30 {
		t.Errorf("Expected 30 transactions, got %d", quality.TxCount)
	}

	if quality.TotalScore == 0 {
		t.Error("Total score should not be zero")
	}

	if quality.RewardMultiplier < 0.1 || quality.RewardMultiplier > 2.0 {
		t.Errorf("Invalid reward multiplier: %f", quality.RewardMultiplier)
	}

	t.Logf("Quality Score: %d, Multiplier: %f", quality.TotalScore, quality.RewardMultiplier)
}

// TestForkChoice tests fork choice rule
func TestForkChoice(t *testing.T) {
	forkChoice := NewForkChoiceRule()

	// Create two blocks at the same height
	header1 := &types.Header{
		Number:     big.NewInt(100),
		Time:       1000,
		Difficulty: big.NewInt(1),
	}
	txs1 := make([]*types.Transaction, 10)
	for i := 0; i < 10; i++ {
		txs1[i] = types.NewTransaction(uint64(i), common.HexToAddress("0x1"), big.NewInt(1), 21000, big.NewInt(1), nil)
	}
	block1 := types.NewBlock(header1, &types.Body{Transactions: txs1}, nil, trie.NewStackTrie(nil))

	header2 := &types.Header{
		Number:     big.NewInt(100),
		Time:       1001,
		Difficulty: big.NewInt(1),
	}
	txs2 := make([]*types.Transaction, 20)
	for i := 0; i < 20; i++ {
		txs2[i] = types.NewTransaction(uint64(i), common.HexToAddress("0x2"), big.NewInt(1), 21000, big.NewInt(1), nil)
	}
	block2 := types.NewBlock(header2, &types.Body{Transactions: txs2}, nil, trie.NewStackTrie(nil))

	// Block2 should win (more transactions)
	selected := forkChoice.SelectCanonicalBlock(block1, block2)
	if selected != block2 {
		t.Error("Fork choice should select block with more transactions")
	}

	// Test with same transaction count
	header3 := &types.Header{
		Number:     big.NewInt(100),
		Time:       999, // Earlier timestamp
		Difficulty: big.NewInt(1),
	}
	block3 := types.NewBlock(header3, &types.Body{Transactions: txs1}, nil, trie.NewStackTrie(nil))

	// Block3 should win (earlier timestamp)
	selected = forkChoice.SelectCanonicalBlock(block1, block3)
	if selected != block3 {
		t.Error("Fork choice should select block with earlier timestamp")
	}
}

// TestOnDemandController tests on-demand block production logic
func TestOnDemandController(t *testing.T) {
	config := DefaultConfig()
	controller := NewOnDemandController(config)

	// Test minimum interval enforcement
	lastBlockTime := time.Now()
	pendingTxCount := 10
	pendingGasTotal := uint64(210000)

	// Should not produce immediately
	if controller.ShouldProduceBlock(lastBlockTime, pendingTxCount, pendingGasTotal) {
		t.Error("Should not produce block immediately after last block")
	}

	// Should produce after minimum interval
	lastBlockTime = time.Now().Add(-2 * time.Second)
	if !controller.ShouldProduceBlock(lastBlockTime, pendingTxCount, pendingGasTotal) {
		t.Error("Should produce block after minimum interval with pending transactions")
	}

	// Should force heartbeat after maximum interval
	lastBlockTime = time.Now().Add(-61 * time.Second)
	if !controller.ShouldForceHeartbeat(lastBlockTime) {
		t.Error("Should force heartbeat block after maximum interval")
	}
}

// TestMultiProducerReward tests multi-producer reward calculation
func TestMultiProducerReward(t *testing.T) {
	config := DefaultConfig()
	scorer := NewBlockQualityScorer(config.QualityConfig)
	calculator := NewMultiProducerRewardCalculator(config, scorer)

	// Create candidate blocks
	candidates := make([]*BlockCandidate, 3)

	for i := 0; i < 3; i++ {
		header := &types.Header{
			Number:     big.NewInt(100),
			Time:       uint64(1000 + i*100),
			GasLimit:   30000000,
			GasUsed:    20000000,
			Difficulty: big.NewInt(1),
		}

		txCount := 30 - i*10 // First has 30, second 20, third 10
		txs := make([]*types.Transaction, txCount)
		for j := 0; j < txCount; j++ {
			txs[j] = types.NewTransaction(
				uint64(j),
				common.HexToAddress("0x1234"),
				big.NewInt(1000),
				21000,
				big.NewInt(1000000000),
				nil,
			)
		}

		block := types.NewBlock(header, &types.Body{Transactions: txs}, nil, trie.NewStackTrie(nil))

		candidates[i] = &BlockCandidate{
			Block:      block,
			Producer:   common.HexToAddress("0x" + string(rune('A'+i))),
			ReceivedAt: time.Now().Add(time.Duration(i*100) * time.Millisecond),
		}
	}

	// Calculate rewards
	totalFees := big.NewInt(1e18) // 1 ETH
	rewards := calculator.CalculateRewards(candidates, totalFees)

	if len(rewards) == 0 {
		t.Fatal("No rewards calculated")
	}

	// First candidate should get a reward
	if rewards[0].Reward.Cmp(big.NewInt(0)) <= 0 {
		t.Error("First candidate should receive reward")
	}

	t.Logf("Calculated %d rewards", len(rewards))
	for _, reward := range rewards {
		t.Logf("Rank %d: Speed=%.2f, Quality=%.2f, Final=%.2f, Reward=%s",
			reward.Candidate.Rank,
			reward.SpeedRatio,
			reward.QualityMulti,
			reward.FinalMultiplier,
			reward.Reward.String())
	}
}

// TestReputationSystem tests the reputation system
func TestReputationSystem(t *testing.T) {
	config := DefaultConfig()
	uptimeCalc := NewUptimeCalculator(config.UptimeConfig)
	penaltyMgr := NewPenaltyManager(config.PenaltyConfig)
	repSystem := NewReputationSystem(config.ReputationConfig, uptimeCalc, penaltyMgr)

	address := common.HexToAddress("0x1234567890")

	// Update reputation
	err := repSystem.UpdateReputation(address)
	if err != nil {
		t.Fatalf("Failed to update reputation: %v", err)
	}

	// Get reputation
	rep, err := repSystem.GetReputation(address)
	if err != nil {
		t.Fatalf("Failed to get reputation: %v", err)
	}

	if rep == nil {
		t.Fatal("Reputation should not be nil")
	}

	t.Logf("Reputation Score: %d", rep.ReputationScore)
}

// TestUptimeCalculator tests uptime calculation
func TestUptimeCalculator(t *testing.T) {
	config := DefaultConfig()
	uptimeCalc := NewUptimeCalculator(config.UptimeConfig)

	address := common.HexToAddress("0x1234567890")

	// Record some heartbeats
	for i := 0; i < 5; i++ {
		msg := &HeartbeatMessage{
			NodeID:    address,
			Timestamp: uint64(time.Now().Unix()),
			SGXQuote:  []byte("mock-quote"),
			Signature: []byte("mock-sig"),
		}
		uptimeCalc.RecordHeartbeat(msg)
	}

	// Calculate uptime score with network statistics
	// For testing, use sample network stats
	networkObservers := 10
	networkTotalTxs := uint64(1000)
	networkTotalGas := uint64(30000000)
	
	uptimeData := uptimeCalc.CalculateUptimeScore(address, networkObservers, networkTotalTxs, networkTotalGas)

	if uptimeData.HeartbeatScore == 0 {
		t.Error("Heartbeat score should not be zero after recording heartbeats")
	}

	t.Logf("Uptime Data: Heartbeat=%d, Consensus=%d, TxParticipation=%d, Response=%d, Comprehensive=%d",
		uptimeData.HeartbeatScore,
		uptimeData.ConsensusScore,
		uptimeData.TxParticipationScore,
		uptimeData.ResponseScore,
		uptimeData.ComprehensiveScore)
}

// TestPenaltyManager tests penalty management
func TestPenaltyManager(t *testing.T) {
	config := DefaultConfig()
	penaltyMgr := NewPenaltyManager(config.PenaltyConfig)

	address := common.HexToAddress("0x1234567890")

	// Record penalties
	for i := 0; i < 3; i++ {
		err := penaltyMgr.RecordPenalty(address, "low_quality", big.NewInt(1e18), "Low quality block")
		if err != nil {
			t.Fatalf("Failed to record penalty: %v", err)
		}
	}

	// Check if excluded
	if !penaltyMgr.IsExcluded(address) {
		t.Error("Node should be excluded after 3 penalties")
	}

	// Get penalty count
	count, err := penaltyMgr.GetPenaltyCount(address)
	if err != nil {
		t.Fatalf("Failed to get penalty count: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 penalties, got %d", count)
	}
}

// BenchmarkBlockQualityScoring benchmarks quality scoring
func BenchmarkBlockQualityScoring(b *testing.B) {
	config := DefaultConfig().QualityConfig
	scorer := NewBlockQualityScorer(config)

	header := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		GasUsed:    24000000,
		Difficulty: big.NewInt(1),
	}

	txs := make([]*types.Transaction, 100)
	for i := 0; i < 100; i++ {
		txs[i] = types.NewTransaction(
			uint64(i),
			common.HexToAddress("0x1234"),
			big.NewInt(1000),
			21000,
			big.NewInt(1000000000),
			nil,
		)
	}

	block := types.NewBlock(header, &types.Body{Transactions: txs}, nil, trie.NewStackTrie(nil))

	b.ResetTimer()
	for range b.N {
		scorer.CalculateQuality(block)
	}
}
