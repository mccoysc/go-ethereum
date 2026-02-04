package sgx

// InstanceID represents a unique hardware identifier for an SGX CPU.
// This is extracted from the SGX Quote and is unique per physical SGX CPU.
type InstanceID struct {
	// CPUInstanceID is the unique identifier for the SGX CPU
	CPUInstanceID []byte

	// QuoteType indicates whether this is EPID or DCAP quote
	QuoteType uint16
}
