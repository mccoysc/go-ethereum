package sgx

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
)

// SGX constants from Gramine
const (
	PAGESIZE       = 4096
	SSA_FRAME_SIZE = 16384 // PAGESIZE * 4
	TCS_SIZE       = 4096
	
	// Page flags
	SGX_SECINFO_R   = 0x1
	SGX_SECINFO_W   = 0x2
	SGX_SECINFO_X   = 0x4
	SGX_SECINFO_TCS = 0x100
	SGX_SECINFO_REG = 0x200
)

// MREnclaveCalculator implements Gramine's MRENCLAVE calculation algorithm
type MREnclaveCalculator struct {
	hash hash.Hash
}

// NewMREnclaveCalculator creates a new calculator
func NewMREnclaveCalculator() *MREnclaveCalculator {
	return &MREnclaveCalculator{
		hash: sha256.New(),
	}
}

// do_ecreate performs ECREATE operation
// Format: struct.pack('<8sLQ44s', b'ECREATE\0', SSA_FRAME_SIZE//PAGESIZE, size, b'\0'*44)
func (m *MREnclaveCalculator) do_ecreate(size uint64) {
	buf := make([]byte, 64)
	
	// "ECREATE\0" (8 bytes)
	copy(buf[0:8], []byte("ECREATE\x00"))
	
	// SSA_FRAME_SIZE / PAGESIZE (4 bytes, little-endian uint32)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(SSA_FRAME_SIZE/PAGESIZE))
	
	// size (8 bytes, little-endian uint64)
	binary.LittleEndian.PutUint64(buf[12:20], size)
	
	// padding (44 bytes of zeros)
	// Already zero from make()
	
	m.hash.Write(buf)
}

// do_eadd performs EADD operation
// Format: struct.pack('<8sQQ40s', b'EADD\0\0\0\0', offset, flags, b'\0'*40)
func (m *MREnclaveCalculator) do_eadd(offset uint64, flags uint64) {
	buf := make([]byte, 64)
	
	// "EADD\0\0\0\0" (8 bytes)
	copy(buf[0:8], []byte("EADD\x00\x00\x00\x00"))
	
	// offset (8 bytes, little-endian uint64)
	binary.LittleEndian.PutUint64(buf[8:16], offset)
	
	// flags (8 bytes, little-endian uint64)
	binary.LittleEndian.PutUint64(buf[16:24], flags)
	
	// padding (40 bytes of zeros)
	// Already zero from make()
	
	m.hash.Write(buf)
}

// do_eextend performs EEXTEND operation
// Format: struct.pack('<8sQ48s', b'EEXTEND\0', offset, b'\0'*48) + content (256 bytes)
func (m *MREnclaveCalculator) do_eextend(offset uint64, content []byte) {
	if len(content) != 256 {
		panic(fmt.Sprintf("EEXTEND content must be 256 bytes, got %d", len(content)))
	}
	
	buf := make([]byte, 64+256)
	
	// "EEXTEND\0" (8 bytes)
	copy(buf[0:8], []byte("EEXTEND\x00"))
	
	// offset (8 bytes, little-endian uint64)
	binary.LittleEndian.PutUint64(buf[8:16], offset)
	
	// padding (48 bytes of zeros)
	// Already zero from make()
	
	// content (256 bytes)
	copy(buf[64:], content)
	
	m.hash.Write(buf)
}

// Result returns the final MRENCLAVE value
func (m *MREnclaveCalculator) Result() []byte {
	return m.hash.Sum(nil)
}

// Memory area structure
type MemoryArea struct {
	Name      string
	Offset    uint64
	Size      uint64
	Flags     uint64
	Measured  bool
	Content   []byte
}

// CalculateMREnclaveFromManifest calculates MRENCLAVE following Gramine's algorithm
func CalculateMREnclaveFromManifest(manifest *ManifestConfig) ([]byte, error) {
	calc := NewMREnclaveCalculator()
	
	// Parse enclave size
	enclaveSize, err := parseEnclaveSize(manifest.SGX.EnclaveSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enclave size: %w", err)
	}
	
	// Step 1: ECREATE
	calc.do_ecreate(enclaveSize)
	
	// Step 2: Create memory areas in Gramine's order
	areas := createMemoryAreas(manifest, enclaveSize)
	
	// Step 3: Process each area
	for _, area := range areas {
		// Calculate number of pages
		numPages := area.Size / PAGESIZE
		
		for i := uint64(0); i < numPages; i++ {
			pageOffset := area.Offset + (i * PAGESIZE)
			
			// EADD for this page
			calc.do_eadd(pageOffset, area.Flags)
			
			// EEXTEND if measured
			if area.Measured && area.Content != nil {
				// Process page in 256-byte chunks
				pageStart := i * PAGESIZE
				for chunk := uint64(0); chunk < PAGESIZE; chunk += 256 {
					chunkOffset := pageOffset + chunk
					contentStart := pageStart + chunk
					contentEnd := contentStart + 256
					
					var content []byte
					if contentEnd <= uint64(len(area.Content)) {
						content = area.Content[contentStart:contentEnd]
					} else if contentStart < uint64(len(area.Content)) {
						// Partial content, pad with zeros
						content = make([]byte, 256)
						copy(content, area.Content[contentStart:])
					} else {
						// No content, all zeros
						content = make([]byte, 256)
					}
					
					calc.do_eextend(chunkOffset, content)
				}
			}
		}
	}
	
	return calc.Result(), nil
}

// createMemoryAreas creates memory layout matching Gramine
func createMemoryAreas(manifest *ManifestConfig, enclaveSize uint64) []MemoryArea {
	areas := []MemoryArea{}
	offset := uint64(0)
	
	// 1. Manifest area (simplified - would contain manifest data)
	manifestSize := uint64(PAGESIZE) // Simplified
	areas = append(areas, MemoryArea{
		Name:     "manifest",
		Offset:   offset,
		Size:     manifestSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W,
		Measured: true,
		Content:  make([]byte, manifestSize), // Would contain actual manifest
	})
	offset += manifestSize
	
	// 2. SSA (Save State Area)
	ssaSize := uint64(SSA_FRAME_SIZE * 4) // 4 frames
	areas = append(areas, MemoryArea{
		Name:     "ssa",
		Offset:   offset,
		Size:     ssaSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W,
		Measured: false,
	})
	offset += ssaSize
	
	// 3. TCS (Thread Control Structure)
	tcsSize := uint64(TCS_SIZE)
	areas = append(areas, MemoryArea{
		Name:     "tcs",
		Offset:   offset,
		Size:     tcsSize,
		Flags:    SGX_SECINFO_TCS,
		Measured: false,
	})
	offset += tcsSize
	
	// 4. TLS (Thread Local Storage)
	tlsSize := uint64(PAGESIZE)
	areas = append(areas, MemoryArea{
		Name:     "tls",
		Offset:   offset,
		Size:     tlsSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W,
		Measured: false,
	})
	offset += tlsSize
	
	// 5. Stack
	stackSize := uint64(PAGESIZE * 256) // 1MB
	areas = append(areas, MemoryArea{
		Name:     "stack",
		Offset:   offset,
		Size:     stackSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W,
		Measured: false,
	})
	offset += stackSize
	
	// 6. Signal stack
	sigStackSize := uint64(PAGESIZE * 16) // 64KB
	areas = append(areas, MemoryArea{
		Name:     "sig_stack",
		Offset:   offset,
		Size:     sigStackSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W,
		Measured: false,
	})
	offset += sigStackSize
	
	// 7. libpal (simplified - would load actual ELF)
	libpalSize := uint64(PAGESIZE * 100) // Simplified
	areas = append(areas, MemoryArea{
		Name:     "libpal",
		Offset:   offset,
		Size:     libpalSize,
		Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_X,
		Measured: true,
		Content:  make([]byte, libpalSize), // Would contain actual libpal.so
	})
	offset += libpalSize
	
	// 8. Free area (rest of enclave)
	if offset < enclaveSize {
		freeSize := enclaveSize - offset
		// Round down to page boundary
		freeSize = (freeSize / PAGESIZE) * PAGESIZE
		if freeSize > 0 {
			areas = append(areas, MemoryArea{
				Name:     "free",
				Offset:   offset,
				Size:     freeSize,
				Flags:    SGX_SECINFO_REG | SGX_SECINFO_R | SGX_SECINFO_W | SGX_SECINFO_X,
				Measured: false,
			})
		}
	}
	
	return areas
}

// parseEnclaveSize parses size string like "2G", "512M"
func parseEnclaveSize(sizeStr string) (uint64, error) {
	if len(sizeStr) == 0 {
		return 0, fmt.Errorf("empty size string")
	}
	
	// Simple parser for common formats
	multiplier := uint64(1)
	numStr := sizeStr
	
	lastChar := sizeStr[len(sizeStr)-1]
	if lastChar == 'G' || lastChar == 'g' {
		multiplier = 1024 * 1024 * 1024
		numStr = sizeStr[:len(sizeStr)-1]
	} else if lastChar == 'M' || lastChar == 'm' {
		multiplier = 1024 * 1024
		numStr = sizeStr[:len(sizeStr)-1]
	} else if lastChar == 'K' || lastChar == 'k' {
		multiplier = 1024
		numStr = sizeStr[:len(sizeStr)-1]
	}
	
	var num uint64
	_, err := fmt.Sscanf(numStr, "%d", &num)
	if err != nil {
		return 0, err
	}
	
	return num * multiplier, nil
}
