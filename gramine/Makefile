# Makefile for building Gramine manifest for X Chain geth
# Based on Gramine official examples

ARCH_LIBDIR ?= /lib/x86_64-linux-gnu

# Configuration parameters
GOVERNANCE_CONTRACT ?= 0xd9145CCE52D386f254917e481eB44e9943F39138
SECURITY_CONFIG_CONTRACT ?= 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045
LOG_LEVEL ?= warning
DEBUG ?= true
SEAL_KEY ?= _sgx_mrsigner
ATTESTATION_TYPE ?= dcap
ENTRYPOINT ?= /app/geth

ifeq ($(DEBUG),1)
DEBUG := true
else
DEBUG := false
endif

.PHONY: all
all: geth.manifest.sgx geth.sig geth.token

# Generate manifest from template
geth.manifest: geth.manifest.template.new
	gramine-manifest \
		-Dlog_level=$(LOG_LEVEL) \
		-Darch_libdir=$(ARCH_LIBDIR) \
		-Dgovernance_contract=$(GOVERNANCE_CONTRACT) \
		-Dsecurity_config_contract=$(SECURITY_CONFIG_CONTRACT) \
		-Ddebug=$(DEBUG) \
		-Dseal_key=$(SEAL_KEY) \
		-Dattestation_type=$(ATTESTATION_TYPE) \
		-Dentrypoint=$(ENTRYPOINT) \
		$< $@

# Generate signing key if not exists
enclave-key.pem:
	openssl genrsa -3 -out $@ 3072

# Sign manifest and generate SIGSTRUCT
geth.manifest.sgx geth.sig: geth.manifest enclave-key.pem
	gramine-sgx-sign \
		--manifest $< \
		--output $@

# Generate token (for DCAP)
geth.token: geth.sig
	gramine-sgx-get-token \
		--output $@ --sig $<

# Extract MRENCLAVE
.PHONY: mrenclave
mrenclave: geth.manifest.sgx
	@gramine-sgx-sigstruct-view $< | grep mr_enclave | awk '{print $$2}'

.PHONY: clean
clean:
	$(RM) geth.manifest geth.manifest.sgx geth.sig geth.token

.PHONY: distclean
distclean: clean
	$(RM) enclave-key.pem
