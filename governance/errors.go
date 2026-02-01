// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package governance

import "errors"

// Bootstrap errors
var (
	ErrBootstrapEnded             = errors.New("bootstrap phase has ended")
	ErrInvalidMREnclave           = errors.New("invalid MRENCLAVE")
	ErrInvalidQuote               = errors.New("invalid SGX quote")
	ErrHardwareAlreadyRegistered  = errors.New("hardware ID already registered")
	ErrMaxFoundersReached         = errors.New("maximum number of founders reached")
)

// Whitelist errors
var (
	ErrMREnclaveNotFound     = errors.New("MRENCLAVE not found")
	ErrMREnclaveNotAllowed   = errors.New("MRENCLAVE not allowed")
	ErrInvalidPermissionLevel = errors.New("invalid permission level")
)

// Voting errors
var (
	ErrProposalNotFound      = errors.New("proposal not found")
	ErrProposalNotPending    = errors.New("proposal is not in pending status")
	ErrAlreadyVoted          = errors.New("voter has already voted on this proposal")
	ErrInvalidVoter          = errors.New("voter is not authorized")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrVotingPeriodEnded     = errors.New("voting period has ended")
	ErrProposalNotPassed     = errors.New("proposal has not passed")
	ErrExecutionDelayNotMet  = errors.New("execution delay not met")
	ErrProposalAlreadyExecuted = errors.New("proposal already executed")
)

// Validator errors
var (
	ErrValidatorNotFound       = errors.New("validator not found")
	ErrInsufficientStake       = errors.New("insufficient stake amount")
	ErrValidatorNotActive      = errors.New("validator is not active")
	ErrInsufficientBalance     = errors.New("insufficient balance")
)

// Admission errors
var (
	ErrAdmissionDenied        = errors.New("admission denied")
	ErrQuoteVerificationFailed = errors.New("quote verification failed")
	ErrNodeNotFound            = errors.New("node not found")
)

// Upgrade errors
var (
	ErrUpgradeReadOnlyMode = errors.New("node is in upgrade read-only mode, write operations are rejected")
)
