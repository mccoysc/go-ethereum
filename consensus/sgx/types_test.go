package sgx

import (
	"testing"
)

func TestSGXExtraEncodeDecode(t *testing.T) {
	original := &SGXExtra{
		SGXQuote:      []byte{1, 2, 3, 4, 5},
		ProducerID:    []byte{0x8a, 0x78, 0x44, 0x3c, 0x14, 0x4d, 0x86, 0xc9, 0x81, 0x15, 0x09, 0x83, 0x9a, 0xb6, 0x0d, 0xfe, 0x9a, 0x31, 0xe1, 0x29, 0xfb, 0xda, 0x1f, 0xe2, 0x60, 0x4b, 0x11, 0xbe, 0x63, 0x3f, 0x7b, 0xfb},
		AttestationTS: 1234567890,
		Signature:     []byte{6, 7, 8, 9, 10},
	}

	// Encode
	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded length: %d bytes", len(encoded))

	// Decode
	decoded, err := DecodeSGXExtra(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Compare
	t.Logf("Original ProducerID: %x", original.ProducerID)
	t.Logf("Decoded ProducerID:  %x", decoded.ProducerID)

	if len(decoded.ProducerID) != len(original.ProducerID) {
		t.Errorf("ProducerID length mismatch: got %d, want %d", len(decoded.ProducerID), len(original.ProducerID))
	}

	for i := range original.ProducerID {
		if decoded.ProducerID[i] != original.ProducerID[i] {
			t.Errorf("ProducerID byte %d mismatch: got %x, want %x", i, decoded.ProducerID[i], original.ProducerID[i])
		}
	}

	if decoded.AttestationTS != original.AttestationTS {
		t.Errorf("AttestationTS mismatch: got %d, want %d", decoded.AttestationTS, original.AttestationTS)
	}
}
