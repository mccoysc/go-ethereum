package sgx

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	// MinExtraDataLength 最小 Extra 数据长度
	MinExtraDataLength = 32
	// MaxExtraDataLength 最大 Extra 数据长度
	MaxExtraDataLength = 1024 * 10 // 10KB
)

// Verifier 区块验证器
type BlockVerifier struct {
	engine *SGXEngine
}

// NewBlockVerifier 创建区块验证器
func NewBlockVerifier(engine *SGXEngine) *BlockVerifier {
	return &BlockVerifier{
		engine: engine,
	}
}

// VerifyBlock 验证完整区块
func (v *BlockVerifier) VerifyBlock(chain consensus.ChainHeaderReader, block *types.Block) error {
	// 验证区块头
	if err := v.engine.verifyHeader(chain, block.Header(), nil); err != nil {
		return err
	}

	// 验证区块体
	if err := v.verifyBody(block); err != nil {
		return err
	}

	return nil
}

// verifyBody 验证区块体
func (v *BlockVerifier) verifyBody(block *types.Block) error {
	// 验证交易数量
	if len(block.Transactions()) > v.engine.config.MaxTxPerBlock {
		return fmt.Errorf("too many transactions: %d > %d",
			len(block.Transactions()), v.engine.config.MaxTxPerBlock)
	}

	// 验证 Gas 总量
	totalGas := uint64(0)
	for _, tx := range block.Transactions() {
		totalGas += tx.Gas()
	}
	if totalGas > v.engine.config.MaxGasPerBlock {
		return fmt.Errorf("total gas exceeds limit: %d > %d",
			totalGas, v.engine.config.MaxGasPerBlock)
	}

	// 验证叔块（PoA-SGX 不允许叔块）
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed in PoA-SGX")
	}

	return nil
}

// verifyBasic 基本验证
func (e *SGXEngine) verifyBasic(header *types.Header) error {
	// 验证区块号
	if header.Number == nil {
		return errors.New("block number is nil")
	}

	// 验证 Extra 字段长度
	if len(header.Extra) < MinExtraDataLength {
		return fmt.Errorf("extra data too short: %d < %d",
			len(header.Extra), MinExtraDataLength)
	}

	if len(header.Extra) > MaxExtraDataLength {
		return fmt.Errorf("extra data too long: %d > %d",
			len(header.Extra), MaxExtraDataLength)
	}

	return nil
}

// verifyTimestamp 验证时间戳
func (e *SGXEngine) verifyTimestamp(header, parent *types.Header) error {
	// 区块时间必须大于父区块
	if header.Time <= parent.Time {
		return errors.New("block timestamp not greater than parent")
	}

	// 区块时间不能太超前
	if header.Time > uint64(time.Now().Add(15*time.Second).Unix()) {
		return ErrFutureBlock
	}

	return nil
}

// verifyQuote 验证 SGX Quote
func (e *SGXEngine) verifyQuote(header *types.Header) error {
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return fmt.Errorf("failed to decode extra data: %w", err)
	}

	// 验证 Quote
	if err := e.verifier.VerifyQuote(extra.SGXQuote); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSGXQuote, err)
	}

	// 验证 Quote 中的 ProducerID
	producerID, err := e.verifier.ExtractProducerID(extra.SGXQuote)
	if err != nil {
		return fmt.Errorf("failed to extract producer ID: %w", err)
	}

	if !bytes.Equal(producerID, extra.ProducerID) {
		return fmt.Errorf("producer ID mismatch: quote=%x, extra=%x",
			producerID, extra.ProducerID)
	}

	// 验证证明时间戳（不能太旧）
	now := uint64(time.Now().Unix())
	maxAge := uint64(3600) // 1小时
	if now > extra.AttestationTS+maxAge {
		return ErrAttestationTooOld
	}

	return nil
}

// verifySignatureInternal 验证区块签名
func (e *SGXEngine) verifySignatureInternal(header *types.Header) error {
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return err
	}

	// 计算签名哈希
	sigHash := e.SealHash(header)

	// 验证签名
	if err := e.verifier.VerifySignature(sigHash.Bytes(), extra.Signature, extra.ProducerID); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	return nil
}

// VerifyHeaderChain 验证区块头链
func (e *SGXEngine) VerifyHeaderChain(chain consensus.ChainHeaderReader, headers []*types.Header) error {
	for i, header := range headers {
		var parent *types.Header
		if i > 0 {
			parent = headers[i-1]
		}

		if err := e.verifyHeader(chain, header, parent); err != nil {
			return fmt.Errorf("header %d validation failed: %w", i, err)
		}
	}

	return nil
}
