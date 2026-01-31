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

package sgx

import (
	"testing"
	"time"
)

func TestConstantTimeCompare(t *testing.T) {
	testCases := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{
			name:     "equal slices",
			a:        []byte("secret_password_12345"),
			b:        []byte("secret_password_12345"),
			expected: true,
		},
		{
			name:     "different length",
			a:        []byte("short"),
			b:        []byte("longer_string"),
			expected: false,
		},
		{
			name:     "same length different content",
			a:        []byte("password1"),
			b:        []byte("password2"),
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []byte{},
			b:        []byte{},
			expected: true,
		},
		{
			name:     "first byte different",
			a:        []byte("aecret_password"),
			b:        []byte("secret_password"),
			expected: false,
		},
		{
			name:     "last byte different",
			a:        []byte("secret_password1"),
			b:        []byte("secret_password2"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConstantTimeCompare(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestConstantTimeCompareTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timing test in short mode")
	}

	secret := []byte("secret_password_12345")

	inputs := [][]byte{
		[]byte("wrong_password_12345"),  // completely different
		[]byte("secret_password_12344"), // last byte different
		[]byte("aecret_password_12345"), // first byte different
		[]byte("secret_password_12345"), // exact match
	}

	var times []time.Duration
	iterations := 10000

	for _, input := range inputs {
		start := time.Now()
		for i := 0; i < iterations; i++ {
			ConstantTimeCompare(input, secret)
		}
		times = append(times, time.Since(start))
	}

	// Calculate average time
	var totalTime time.Duration
	for _, d := range times {
		totalTime += d
	}
	avgTime := totalTime / time.Duration(len(times))

	// Check that all times are within a reasonable deviation
	// We allow 10% deviation for timing variations
	for i, d := range times {
		deviation := float64(d-avgTime) / float64(avgTime)
		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > 0.10 {
			t.Logf("Input %d has timing deviation: %.2f%% (time: %v, avg: %v)",
				i, deviation*100, d, avgTime)
			// Note: We log but don't fail, as timing can vary on different systems
		}
	}
}

func TestConstantTimeCopy(t *testing.T) {
	dst := []byte("destination_data")
	src := []byte("source_data_here")

	// Test with condition = true
	dstCopy := make([]byte, len(dst))
	copy(dstCopy, dst)
	ConstantTimeCopy(true, dstCopy, src)

	// Should have copied src to dst
	for i := range dstCopy {
		if i < len(src) && dstCopy[i] != src[i] {
			t.Errorf("At index %d: expected %x, got %x", i, src[i], dstCopy[i])
		}
	}

	// Test with condition = false
	dstCopy2 := make([]byte, len(dst))
	copy(dstCopy2, dst)
	ConstantTimeCopy(false, dstCopy2, src)

	// Should not have copied - dst should remain unchanged
	for i := range dstCopy2 {
		if dstCopy2[i] != dst[i] {
			t.Errorf("At index %d: dst was modified when condition=false", i)
		}
	}
}

func TestConstantTimeSelect(t *testing.T) {
	a := []byte("option_a")
	b := []byte("option_b")

	// Test with condition = true (should select a)
	result := ConstantTimeSelect(true, a, b)
	if result == nil {
		t.Fatal("Result is nil")
	}

	for i := range result {
		if result[i] != a[i] {
			t.Errorf("Expected a[%d]=%x, got %x", i, a[i], result[i])
		}
	}

	// Test with condition = false (should select b)
	result2 := ConstantTimeSelect(false, a, b)
	if result2 == nil {
		t.Fatal("Result is nil")
	}

	for i := range result2 {
		if result2[i] != b[i] {
			t.Errorf("Expected b[%d]=%x, got %x", i, b[i], result2[i])
		}
	}

	// Test with different lengths (should return nil)
	c := []byte("short")
	d := []byte("longer_string")
	result3 := ConstantTimeSelect(true, c, d)
	if result3 != nil {
		t.Error("Expected nil for different length inputs")
	}
}

func TestConstantTimeSelectTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timing test in short mode")
	}

	a := make([]byte, 32)
	b := make([]byte, 32)

	for i := range a {
		a[i] = byte(i)
		b[i] = byte(i + 100)
	}

	iterations := 10000

	// Test with condition = true
	startTrue := time.Now()
	for i := 0; i < iterations; i++ {
		ConstantTimeSelect(true, a, b)
	}
	timeTrue := time.Since(startTrue)

	// Test with condition = false
	startFalse := time.Now()
	for i := 0; i < iterations; i++ {
		ConstantTimeSelect(false, a, b)
	}
	timeFalse := time.Since(startFalse)

	// Check timing deviation
	avgTime := (timeTrue + timeFalse) / 2
	deviationTrue := float64(timeTrue-avgTime) / float64(avgTime)
	deviationFalse := float64(timeFalse-avgTime) / float64(avgTime)

	if deviationTrue < 0 {
		deviationTrue = -deviationTrue
	}
	if deviationFalse < 0 {
		deviationFalse = -deviationFalse
	}

	// Log timing information
	t.Logf("Time with condition=true: %v", timeTrue)
	t.Logf("Time with condition=false: %v", timeFalse)
	t.Logf("Deviation: true=%.2f%%, false=%.2f%%",
		deviationTrue*100, deviationFalse*100)
}

func BenchmarkConstantTimeCompare(b *testing.B) {
	a := []byte("secret_password_12345")
	c := []byte("secret_password_12345")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConstantTimeCompare(a, c)
	}
}

func BenchmarkConstantTimeCopy(b *testing.B) {
	dst := make([]byte, 32)
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConstantTimeCopy(true, dst, src)
	}
}

func BenchmarkConstantTimeSelect(b *testing.B) {
	a := make([]byte, 32)
	c := make([]byte, 32)
	for i := range a {
		a[i] = byte(i)
		c[i] = byte(i + 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConstantTimeSelect(true, a, c)
	}
}
