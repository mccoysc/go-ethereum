package sgx

import (
	"errors"
)

var (
	// 通用错误
	ErrInvalidConfig   = errors.New("invalid configuration")
	ErrUnknownAncestor = errors.New("unknown ancestor")
	ErrInvalidBlock    = errors.New("invalid block")
	ErrInvalidHeader   = errors.New("invalid header")
	ErrInvalidExtra    = errors.New("invalid extra data")

	// SGX 相关错误
	ErrInvalidSGXQuote         = errors.New("invalid SGX quote")
	ErrInvalidProducerID       = errors.New("invalid producer ID")
	ErrInvalidSignature        = errors.New("invalid signature")
	ErrQuoteVerificationFailed = errors.New("SGX quote verification failed")
	ErrAttestationTooOld       = errors.New("attestation timestamp too old")

	// 验证错误
	ErrFutureBlock       = errors.New("block timestamp too far in future")
	ErrInvalidTimestamp  = errors.New("invalid timestamp")
	ErrInvalidDifficulty = errors.New("invalid difficulty")
	ErrInvalidMixDigest  = errors.New("invalid mix digest")
	ErrInvalidNonce      = errors.New("invalid nonce")

	// 出块错误
	ErrNoTransactions        = errors.New("no transactions to include")
	ErrTooManyTransactions   = errors.New("too many transactions")
	ErrBlockIntervalTooShort = errors.New("block interval too short")

	// 奖励错误
	ErrInvalidReward    = errors.New("invalid reward calculation")
	ErrNoRewardData     = errors.New("no reward data available")
	ErrServiceNotFound  = errors.New("service not found")

	// 信誉错误
	ErrNodeExcluded  = errors.New("node is excluded due to penalties")
	ErrLowReputation = errors.New("node reputation too low")

	// 停止信号
	ErrStopped = errors.New("operation stopped")
)
