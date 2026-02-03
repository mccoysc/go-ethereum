// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Based on Gramine's official SIGSTRUCT implementation
// Reference: https://github.com/gramineproject/gramine
//   - pal/src/host/linux-sgx/sgx_arch.h
//   - python/graminelibos/sigstruct.py

package sgx

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

// SIGSTRUCT offsets based on Gramine's sgx_arch.h
// typedef struct {
//     uint8_t header[16];                  // offset 0
//     uint32_t vendor;                     // offset 16
//     uint32_t date;                       // offset 20
//     uint8_t header2[16];                 // offset 24
//     uint32_t swdefined;                  // offset 40
//     uint8_t reserved1[84];               // offset 44
//     uint8_t modulus[384];                // offset 128 ← RSA-N (SE_KEY_SIZE)
//     uint8_t exponent[4];                 // offset 512 ← RSA-e (SE_EXPONENT_SIZE)
//     uint8_t signature[384];              // offset 516 ← RSA-S (SE_KEY_SIZE)
//     sgx_misc_select_t misc_select;       // offset 900 (4 bytes)
//     sgx_misc_select_t misc_mask;         // offset 904 (4 bytes)
//     sgx_cet_attributes_t cet_attributes; // offset 908 (1 byte)
//     sgx_cet_attributes_t cet_attributes_mask; // offset 909 (1 byte)
//     uint8_t reserved2[2];                // offset 910
//     sgx_isvfamily_id_t isv_family_id;    // offset 912 (16 bytes)
//     sgx_attributes_t attributes;         // offset 928 (16 bytes)
//     sgx_attributes_t attribute_mask;     // offset 944 (16 bytes)
//     sgx_measurement_t enclave_hash;      // offset 960 ← MRENCLAVE (32 bytes)
//     uint8_t reserved3[16];               // offset 992
//     sgx_isvext_prod_id_t isvext_prod_id; // offset 1008 (16 bytes)
//     sgx_prod_id_t isv_prod_id;           // offset 1024 (2 bytes)
//     sgx_isv_svn_t isv_svn;               // offset 1026 (2 bytes)
//     uint8_t reserved4[12];               // offset 1028
//     uint8_t q1[384];                     // offset 1040 (SE_KEY_SIZE)
//     uint8_t q2[384];                     // offset 1424 (SE_KEY_SIZE)
// } sgx_sigstruct_t;  // Total: 1808 bytes

// Additional constants for SIGSTRUCT (complements manifest_verify_mrenclave.go)
const (
	sigstructExponentOffset   = 512
	sigstructSignatureOffset  = 516
	sigstructMiscSelectOffset = 900
	sigstructQ1Offset         = 1040
	sigstructQ2Offset         = 1424

	rsaKeySize      = 384 // 3072 bits = 384 bytes
	rsaExponentSize = 4

	// SGX requires RSA exponent to be 3
	sgxRSAExponent = 3
)

// SIGSTRUCT represents Intel SGX SIGSTRUCT structure
type SIGSTRUCT struct {
	Header      [16]byte
	Vendor      uint32
	Date        uint32
	Header2     [16]byte
	SwDefined   uint32
	Modulus     [384]byte  // RSA-N
	Exponent    uint32     // RSA-e (must be 3)
	Signature   [384]byte  // RSA-S
	MiscSelect  uint32
	MiscMask    uint32
	Attributes  [16]byte
	AttrMask    [16]byte
	MREnclave   [32]byte   // enclave_hash
	ISVProdID   uint16
	ISVSVN      uint16
	Q1          [384]byte
	Q2          [384]byte
}

// ParseSIGSTRUCT parses a SIGSTRUCT from bytes
func ParseSIGSTRUCT(data []byte) (*SIGSTRUCT, error) {
	if len(data) < sigstructSize {
		return nil, fmt.Errorf("SIGSTRUCT data too small: %d bytes (expected %d)", len(data), sigstructSize)
	}

	sig := &SIGSTRUCT{}

	// Extract fields
	copy(sig.Header[:], data[0:16])
	sig.Vendor = binary.LittleEndian.Uint32(data[16:20])
	sig.Date = binary.LittleEndian.Uint32(data[20:24])
	copy(sig.Header2[:], data[24:40])
	sig.SwDefined = binary.LittleEndian.Uint32(data[40:44])

	copy(sig.Modulus[:], data[sigstructModulusOffset:sigstructModulusOffset+rsaKeySize])
	sig.Exponent = binary.LittleEndian.Uint32(data[sigstructExponentOffset:sigstructExponentOffset+rsaExponentSize])
	copy(sig.Signature[:], data[sigstructSignatureOffset:sigstructSignatureOffset+rsaKeySize])

	sig.MiscSelect = binary.LittleEndian.Uint32(data[sigstructMiscSelectOffset:sigstructMiscSelectOffset+4])
	sig.MiscMask = binary.LittleEndian.Uint32(data[sigstructMiscSelectOffset+4:sigstructMiscSelectOffset+8])

	copy(sig.Attributes[:], data[928:944])
	copy(sig.AttrMask[:], data[944:960])
	copy(sig.MREnclave[:], data[sigstructMREnclaveOffset:sigstructMREnclaveOffset+mrenclaveSize])

	sig.ISVProdID = binary.LittleEndian.Uint16(data[1024:1026])
	sig.ISVSVN = binary.LittleEndian.Uint16(data[1026:1028])

	copy(sig.Q1[:], data[sigstructQ1Offset:sigstructQ1Offset+rsaKeySize])
	copy(sig.Q2[:], data[sigstructQ2Offset:sigstructQ2Offset+rsaKeySize])

	return sig, nil
}

// GetSigningData extracts the data that was signed in SIGSTRUCT
// According to Gramine's implementation:
//   signing_data = bytes[0:128] + bytes[900:1028]  (256 bytes total)
func GetSigningData(sigstructData []byte) ([]byte, error) {
	if len(sigstructData) < sigstructSize {
		return nil, fmt.Errorf("SIGSTRUCT data too small")
	}

	// Extract two parts as per Gramine's get_signing_data()
	part1 := sigstructData[0:128]
	part2 := sigstructData[sigstructMiscSelectOffset : sigstructMiscSelectOffset+128]

	return append(part1, part2...), nil
}

// VerifySIGSTRUCTSignature verifies the RSA signature in SIGSTRUCT
// This implements the reverse of Gramine's signing process
// Reference: Intel SGX SDK sign_tool and Gramine sigstruct.py
func VerifySIGSTRUCTSignature(sigstructData []byte) error {
	sig, err := ParseSIGSTRUCT(sigstructData)
	if err != nil {
		return err
	}

	// Check exponent is 3 (SGX requirement)
	if sig.Exponent != sgxRSAExponent {
		return fmt.Errorf("invalid RSA exponent: %d (expected %d)", sig.Exponent, sgxRSAExponent)
	}

	// Get signing data (256 bytes: first 128 + bytes[900:1028])
	signingData, err := GetSigningData(sigstructData)
	if err != nil {
		return err
	}

	// Hash the signing data with SHA256
	hash := sha256.Sum256(signingData)

	// Convert signature and modulus from little-endian bytes to big.Int
	sigBytes := reverseBytes(sig.Signature[:])
	modBytes := reverseBytes(sig.Modulus[:])

	S := new(big.Int).SetBytes(sigBytes)
	N := new(big.Int).SetBytes(modBytes)

	// Verify modulus size
	if N.BitLen() != 3072 {
		return fmt.Errorf("invalid modulus size: %d bits (expected 3072)", N.BitLen())
	}

	// Verify: S³ mod N should equal the padded hash
	// RSA with e=3: S³ mod N = M (where M is PKCS#1 v1.5 padded hash)
	S3 := new(big.Int).Exp(S, big.NewInt(3), N)

	// Convert S³ mod N to bytes (big-endian)
	paddedMessage := S3.Bytes()

	// Verify PKCS#1 v1.5 padding structure:
	// 0x00 || 0x01 || PS || 0x00 || DigestInfo || Hash
	// Where PS is padding bytes (all 0xFF)
	// DigestInfo for SHA256 is: 30 31 30 0d 06 09 60 86 48 01 65 03 04 02 01 05 00 04 20
	
	if len(paddedMessage) < 11+19+32 { // minimum: 2 + 8 + 1 + 19 + 32
		return fmt.Errorf("invalid padded message length: %d", len(paddedMessage))
	}

	// Check 0x00 0x01 prefix
	if paddedMessage[0] != 0x00 || paddedMessage[1] != 0x01 {
		return fmt.Errorf("invalid PKCS#1 v1.5 padding: wrong prefix %02x %02x", paddedMessage[0], paddedMessage[1])
	}

	// Find the 0x00 separator after padding
	separatorIndex := -1
	for i := 2; i < len(paddedMessage); i++ {
		if paddedMessage[i] == 0x00 {
			separatorIndex = i
			break
		} else if paddedMessage[i] != 0xFF {
			return fmt.Errorf("invalid PKCS#1 v1.5 padding: non-0xFF byte at position %d", i)
		}
	}

	if separatorIndex == -1 {
		return fmt.Errorf("invalid PKCS#1 v1.5 padding: no separator found")
	}

	// DigestInfo for SHA256
	// 30 31 30 0d 06 09 60 86 48 01 65 03 04 02 01 05 00 04 20
	digestInfo := []byte{
		0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86,
		0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05,
		0x00, 0x04, 0x20,
	}

	// Extract DigestInfo + Hash
	dataStart := separatorIndex + 1
	if len(paddedMessage)-dataStart != len(digestInfo)+32 {
		return fmt.Errorf("invalid data length after separator: %d (expected %d)", 
			len(paddedMessage)-dataStart, len(digestInfo)+32)
	}

	// Verify DigestInfo
	for i := 0; i < len(digestInfo); i++ {
		if paddedMessage[dataStart+i] != digestInfo[i] {
			return fmt.Errorf("invalid DigestInfo at byte %d: %02x (expected %02x)",
				i, paddedMessage[dataStart+i], digestInfo[i])
		}
	}

	// Extract and verify hash
	extractedHash := paddedMessage[dataStart+len(digestInfo):]
	if len(extractedHash) != 32 {
		return fmt.Errorf("invalid hash length: %d", len(extractedHash))
	}

	// Compare with computed hash
	if !bytes.Equal(extractedHash, hash[:]) {
		return fmt.Errorf("signature verification failed: hash mismatch")
	}

	log.Info("SIGSTRUCT signature verified successfully",
		"exponent", sig.Exponent,
		"modulus_bits", N.BitLen(),
		"hash", hex.EncodeToString(hash[:]))

	return nil
}

// ExtractMREnclaveFromSIGSTRUCT extracts MRENCLAVE from SIGSTRUCT
func ExtractMREnclaveFromSIGSTRUCT(manifestPath string) ([]byte, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.sgx: %w", err)
	}

	if len(data) < sigstructSize {
		return nil, fmt.Errorf("manifest.sgx file too small: %d bytes", len(data))
	}

	// Extract MRENCLAVE from SIGSTRUCT (offset 960, 32 bytes)
	mrenclave := make([]byte, mrenclaveSize)
	copy(mrenclave, data[sigstructMREnclaveOffset:sigstructMREnclaveOffset+mrenclaveSize])

	// Validate MRENCLAVE is not all zeros
	allZero := true
	for _, b := range mrenclave {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, fmt.Errorf("invalid MRENCLAVE: all zeros")
	}

	log.Debug("Extracted MRENCLAVE from SIGSTRUCT",
		"mrenclave", hex.EncodeToString(mrenclave))

	return mrenclave, nil
}

// VerifyManifestSIGSTRUCT verifies the complete manifest.sgx file
func VerifyManifestSIGSTRUCT(manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.sgx: %w", err)
	}

	if len(data) < sigstructSize {
		return fmt.Errorf("manifest.sgx too small: %d bytes", len(data))
	}

	// Extract SIGSTRUCT (first 1808 bytes)
	sigstructData := data[0:sigstructSize]

	// Verify SIGSTRUCT signature
	if err := VerifySIGSTRUCTSignature(sigstructData); err != nil {
		return fmt.Errorf("SIGSTRUCT signature verification failed: %w", err)
	}

	// Verify MRENCLAVE consistency (build-tag-specific)
	if err := verifyMREnclaveConsistency(manifestPath); err != nil {
		return fmt.Errorf("MRENCLAVE verification failed: %w", err)
	}

	log.Info("Manifest SIGSTRUCT verification successful")
	return nil
}

// reverseBytes reverses a byte slice (for little-endian to big-endian conversion)
func reverseBytes(b []byte) []byte {
	result := make([]byte, len(b))
	for i := range b {
		result[i] = b[len(b)-1-i]
	}
	return result
}
