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

import (
"math/big"
"testing"
"time"
)

func TestDefaultCoreValidatorConfig(t *testing.T) {
config := DefaultCoreValidatorConfig()

if config == nil {
t.Fatal("config should not be nil")
}

if config.MinMembers != 5 {
t.Errorf("expected MinMembers 5, got %d", config.MinMembers)
}

if config.MaxMembers != 7 {
t.Errorf("expected MaxMembers 7, got %d", config.MaxMembers)
}

if config.QuorumThreshold != 0.667 {
t.Errorf("expected QuorumThreshold 0.667, got %f", config.QuorumThreshold)
}
}

func TestDefaultCommunityValidatorConfig(t *testing.T) {
config := DefaultCommunityValidatorConfig()

if config == nil {
t.Fatal("config should not be nil")
}

expectedUptime := 30 * 24 * time.Hour
if config.MinUptime != expectedUptime {
t.Errorf("expected MinUptime %v, got %v", expectedUptime, config.MinUptime)
}

expectedStake := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
if config.MinStake.Cmp(expectedStake) != 0 {
t.Errorf("expected MinStake %v, got %v", expectedStake, config.MinStake)
}

if config.VetoThreshold != 0.334 {
t.Errorf("expected VetoThreshold 0.334, got %f", config.VetoThreshold)
}
}
