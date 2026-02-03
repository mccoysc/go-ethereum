package sgx

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SGXMeasurement represents an SGX measurement context
type SGXMeasurement struct {
	value [32]byte
}

// NewSGXMeasurement creates a new measurement initialized to zero
func NewSGXMeasurement() *SGXMeasurement {
	return &SGXMeasurement{}
}

// Update updates the measurement with new data using SHA256
func (m *SGXMeasurement) Update(data []byte) {
	h := sha256.New()
	h.Write(m.value[:])
	h.Write(data)
	copy(m.value[:], h.Sum(nil))
}

// Value returns the current measurement value
func (m *SGXMeasurement) Value() []byte {
	result := make([]byte, 32)
	copy(result, m.value[:])
	return result
}

// CalculateMREnclave calculates MRENCLAVE from manifest configuration
// This implements a simplified version of SGX measurement algorithm
func CalculateMREnclave(manifest *ManifestConfig, fileHashes map[string][]byte) ([]byte, error) {
	measurement := NewSGXMeasurement()
	
	// Parse enclave size
	enclaveSize, err := parseSize(manifest.SGX.EnclaveSize)
	if err != nil {
		return nil, fmt.Errorf("invalid enclave size: %w", err)
	}
	
	// ECREATE - Initialize enclave
	ecreateData := buildECREATEData(enclaveSize, manifest)
	measurement.Update(ecreateData)
	
	// Add trusted files to measurement
	for _, file := range manifest.SGX.TrustedFiles {
		fileHash, ok := fileHashes[file.URI]
		if !ok {
			// If hash not provided, try to read from manifest
			if file.SHA256 != "" {
				var err error
				fileHash, err = hex.DecodeString(file.SHA256)
				if err != nil {
					return nil, fmt.Errorf("invalid SHA256 for %s: %w", file.URI, err)
				}
			} else {
				// Skip if no hash available
				continue
			}
		}
		
		// EADD and EEXTEND for this file's hash
		eaddData := buildEADDData(fileHash)
		measurement.Update(eaddData)
		
		// EEXTEND - extend measurement with file hash in chunks
		for i := 0; i < len(fileHash); i += 256 {
			end := i + 256
			if end > len(fileHash) {
				// Pad to 256 bytes
				chunk := make([]byte, 256)
				copy(chunk, fileHash[i:])
				measurement.Update(buildEEXTENDData(chunk))
			} else {
				measurement.Update(buildEEXTENDData(fileHash[i:end]))
			}
		}
	}
	
	return measurement.Value(), nil
}

// buildECREATEData builds data for ECREATE operation
func buildECREATEData(size uint64, manifest *ManifestConfig) []byte {
	data := make([]byte, 64)
	
	// Add enclave size
	binary.LittleEndian.PutUint64(data[0:8], size)
	
	// Add SSA frame size (typically 1)
	binary.LittleEndian.PutUint32(data[8:12], 1)
	
	// Add misc select
	if manifest.SGX.MiscSelect != "" {
		// Parse misc_select if provided
		miscSelect := parseMiscSelect(manifest.SGX.MiscSelect)
		binary.LittleEndian.PutUint32(data[12:16], miscSelect)
	}
	
	return data
}

// buildEADDData builds data for EADD operation
func buildEADDData(fileHash []byte) []byte {
	data := make([]byte, 64)
	
	// Include file hash in EADD data
	copy(data[0:32], fileHash)
	
	// Page type and permissions (simplified)
	binary.LittleEndian.PutUint32(data[32:36], 0x02) // PT_REG
	binary.LittleEndian.PutUint32(data[36:40], 0x07) // R+W+X
	
	return data
}

// buildEEXTENDData builds data for EEXTEND operation
func buildEEXTENDData(chunk []byte) []byte {
	// EEXTEND processes 256-byte chunks
	data := make([]byte, 256)
	copy(data, chunk)
	return data
}

// parseSize parses size string like "2G", "1024M", "512K"
func parseSize(sizeStr string) (uint64, error) {
	if sizeStr == "" {
		return 1024 * 1024 * 1024, nil // Default 1GB
	}
	
	sizeStr = strings.TrimSpace(sizeStr)
	if len(sizeStr) < 2 {
		return strconv.ParseUint(sizeStr, 10, 64)
	}
	
	unit := sizeStr[len(sizeStr)-1]
	valueStr := sizeStr[:len(sizeStr)-1]
	
	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, err
	}
	
	switch unit {
	case 'K', 'k':
		return value * 1024, nil
	case 'M', 'm':
		return value * 1024 * 1024, nil
	case 'G', 'g':
		return value * 1024 * 1024 * 1024, nil
	default:
		// No unit, assume bytes
		return strconv.ParseUint(sizeStr, 10, 64)
	}
}

// parseMiscSelect parses misc_select value
func parseMiscSelect(miscStr string) uint32 {
	// Simplified - parse as hex or decimal
	if strings.HasPrefix(miscStr, "0x") {
		val, _ := strconv.ParseUint(miscStr[2:], 16, 32)
		return uint32(val)
	}
	val, _ := strconv.ParseUint(miscStr, 10, 32)
	return uint32(val)
}

// CalculateTrustedFilesHashes reads and calculates hashes for trusted files
func CalculateTrustedFilesHashes(files []TrustedFileEntry) (map[string][]byte, error) {
	hashes := make(map[string][]byte)
	
	for _, file := range files {
		// Try to read file and calculate hash
		content, err := os.ReadFile(file.URI)
		if err != nil {
			// If file doesn't exist, use hash from manifest if available
			if file.SHA256 != "" {
				hash, err := hex.DecodeString(file.SHA256)
				if err != nil {
					return nil, fmt.Errorf("invalid SHA256 for %s: %w", file.URI, err)
				}
				hashes[file.URI] = hash
				continue
			}
			// Skip files that don't exist and don't have hash
			continue
		}
		
		// Calculate SHA256
		hash := sha256.Sum256(content)
		hashes[file.URI] = hash[:]
	}
	
	return hashes, nil
}
