//go:build testenv
// +build testenv

package sgx

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiagnoseRealQuote(t *testing.T) {
	// Load the real quote
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	quotePath := filepath.Join(dir, "testdata", "gramine_ratls_quote.bin")

	quote, err := os.ReadFile(quotePath)
	if err != nil {
		t.Fatalf("Failed to load real quote: %v", err)
	}

	t.Logf("Quote size: %d bytes", len(quote))

	// Check basic structure
	if len(quote) < 436 {
		t.Fatal("Quote too short")
	}

	version := binary.LittleEndian.Uint16(quote[0:2])
	signType := binary.LittleEndian.Uint16(quote[2:4])
	t.Logf("Version: %d, SignType: %d", version, signType)

	// Check signature data
	signatureDataLen := binary.LittleEndian.Uint32(quote[432:436])
	t.Logf("Signature data length: %d", signatureDataLen)

	if len(quote) < 436+int(signatureDataLen) {
		t.Fatal("Quote signature data truncated")
	}

	// Skip to certification data offset
	offset := 436 + 64 + 64 + 384 + 64  // ECDSA sig + pubkey + QE report + QE sig

	if offset+2 > len(quote) {
		t.Fatal("No auth data size field")
	}

	authDataSize := binary.LittleEndian.Uint16(quote[offset : offset+2])
	t.Logf("Auth data size: %d", authDataSize)
	offset += 2 + int(authDataSize)

	if offset+6 > len(quote) {
		t.Fatal("No certification data")
	}

	certDataType := binary.LittleEndian.Uint16(quote[offset : offset+2])
	certDataSize := binary.LittleEndian.Uint32(quote[offset+2 : offset+6])
	t.Logf("Cert data type: %d, Cert data size: %d", certDataType, certDataSize)

	if certDataType == 1 {
		t.Logf("Quote contains PPID (type 1)")
		if offset+6+int(certDataSize) <= len(quote) {
			ppid := quote[offset+6 : offset+6+int(certDataSize)]
			t.Logf("PPID length: %d", len(ppid))
			t.Logf("PPID (first 32 bytes): %x", ppid[:min(32, len(ppid))])
		}
	} else if certDataType == 5 {
		t.Logf("Quote contains PCK certificate chain (type 5)")
	} else {
		t.Logf("Quote contains cert type: %d", certDataType)
	}

	// Now try extracting with the real function
	verifier := NewDCAPVerifier(true)
	instanceID, source, err := verifier.extractPlatformInstanceID(quote)
	if err != nil {
		t.Logf("extractPlatformInstanceID failed: %v", err)
	} else {
		t.Logf("extractPlatformInstanceID succeeded!")
		t.Logf("  Instance ID: %x", instanceID)
		t.Logf("  Source: %s", source)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
