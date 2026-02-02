#!/bin/bash
# Cryptographic test utilities for SGX precompiled contracts

# SGX Precompiled Contract Addresses
SGX_KEY_CREATE="0x0000000000000000000000000000000000008000"
SGX_KEY_GET_PUBLIC="0x0000000000000000000000000000000000008001"
SGX_SIGN="0x0000000000000000000000000000000000008002"
SGX_VERIFY="0x0000000000000000000000000000000000008003"
SGX_ECDH="0x0000000000000000000000000000000000008004"
SGX_RANDOM="0x0000000000000000000000000000000000008005"
SGX_ENCRYPT="0x0000000000000000000000000000000000008006"
SGX_DECRYPT="0x0000000000000000000000000000000000008007"
SGX_KEY_DERIVE="0x0000000000000000000000000000000000008008"
SGX_KEY_DELETE="0x0000000000000000000000000000000000008009"

# Key types
KEY_TYPE_ECDSA=1
KEY_TYPE_ED25519=2
KEY_TYPE_AES256=3

# Create a key using SGX_KEY_CREATE precompiled contract
sgx_create_key() {
    local key_type="$1"
    local from_address="$2"
    local rpc_port="${3:-8545}"
    
    # Encode input: keyType (uint8)
    local input_data="0x$(printf '%064x' $key_type)"
    
    echo "Creating key of type $key_type from $from_address..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_KEY_CREATE" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    echo "Transaction hash: $tx_hash" >&2
    
    # Wait for transaction to be mined
    local receipt=$(wait_for_transaction "$tx_hash" "$rpc_port")
    
    if [ -z "$receipt" ]; then
        echo "Transaction not mined" >&2
        return 1
    fi
    
    # Extract key ID from logs
    local key_id=$(echo "$receipt" | jq -r '.logs[0].data // empty')
    
    if [ -z "$key_id" ]; then
        echo "No key ID in receipt" >&2
        return 1
    fi
    
    echo "$key_id"
}

# Get public key using SGX_KEY_GET_PUBLIC
sgx_get_public_key() {
    local key_id="$1"
    local from_address="$2"
    local rpc_port="${3:-8545}"
    
    # Encode input: keyId (bytes32)
    local input_data="$key_id"
    
    echo "Getting public key for $key_id..." >&2
    
    # Call contract (read-only)
    local result=$(call_precompiled_contract "$SGX_KEY_GET_PUBLIC" "$input_data" "$from_address" "$rpc_port")
    
    echo "$result"
}

# Sign data using SGX_SIGN
sgx_sign() {
    local key_id="$1"
    local message_hash="$2"
    local from_address="$3"
    local rpc_port="${4:-8545}"
    
    # Encode input: keyId (bytes32) + messageHash (bytes32)
    local input_data="${key_id}${message_hash#0x}"
    
    echo "Signing message with key $key_id..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_SIGN" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    # Wait for transaction
    local receipt=$(wait_for_transaction "$tx_hash" "$rpc_port")
    
    if [ -z "$receipt" ]; then
        echo "Transaction not mined" >&2
        return 1
    fi
    
    # Extract signature from logs
    local signature=$(echo "$receipt" | jq -r '.logs[0].data // empty')
    
    echo "$signature"
}

# Verify signature using SGX_VERIFY
sgx_verify() {
    local public_key="$1"
    local message_hash="$2"
    local signature="$3"
    local from_address="$4"
    local rpc_port="${5:-8545}"
    
    # Encode input: publicKey + messageHash + signature
    local input_data="${public_key}${message_hash#0x}${signature#0x}"
    
    echo "Verifying signature..." >&2
    
    # Call contract (read-only)
    local result=$(call_precompiled_contract "$SGX_VERIFY" "$input_data" "$from_address" "$rpc_port")
    
    # Result should be 0x01 for valid signature
    if [ "$result" = "0x0000000000000000000000000000000000000000000000000000000000000001" ]; then
        echo "true"
        return 0
    else
        echo "false"
        return 1
    fi
}

# Delete key using SGX_KEY_DELETE
sgx_delete_key() {
    local key_id="$1"
    local from_address="$2"
    local rpc_port="${3:-8545}"
    
    # Encode input: keyId (bytes32)
    local input_data="$key_id"
    
    echo "Deleting key $key_id..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_KEY_DELETE" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    # Wait for transaction
    wait_for_transaction "$tx_hash" "$rpc_port" > /dev/null
    
    echo "$tx_hash"
}

# Generate random bytes using SGX_RANDOM
sgx_random() {
    local num_bytes="$1"
    local from_address="$2"
    local rpc_port="${3:-8545}"
    
    # Encode input: numBytes (uint256)
    local input_data="0x$(printf '%064x' $num_bytes)"
    
    echo "Generating $num_bytes random bytes..." >&2
    
    # Call contract (read-only)
    local result=$(call_precompiled_contract "$SGX_RANDOM" "$input_data" "$from_address" "$rpc_port")
    
    echo "$result"
}

# Encrypt data using SGX_ENCRYPT
sgx_encrypt() {
    local key_id="$1"
    local plaintext="$2"
    local from_address="$3"
    local rpc_port="${4:-8545}"
    
    # Encode input: keyId + plaintext
    local input_data="${key_id}${plaintext#0x}"
    
    echo "Encrypting data with key $key_id..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_ENCRYPT" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    # Wait for transaction
    local receipt=$(wait_for_transaction "$tx_hash" "$rpc_port")
    
    # Extract ciphertext from logs
    local ciphertext=$(echo "$receipt" | jq -r '.logs[0].data // empty')
    
    echo "$ciphertext"
}

# Decrypt data using SGX_DECRYPT
sgx_decrypt() {
    local key_id="$1"
    local ciphertext="$2"
    local from_address="$3"
    local rpc_port="${4:-8545}"
    
    # Encode input: keyId + ciphertext
    local input_data="${key_id}${ciphertext#0x}"
    
    echo "Decrypting data with key $key_id..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_DECRYPT" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    # Wait for transaction
    local receipt=$(wait_for_transaction "$tx_hash" "$rpc_port")
    
    # Extract plaintext from logs
    local plaintext=$(echo "$receipt" | jq -r '.logs[0].data // empty')
    
    echo "$plaintext"
}

# Perform ECDH key exchange using SGX_ECDH
sgx_ecdh() {
    local private_key_id="$1"
    local public_key="$2"
    local from_address="$3"
    local rpc_port="${4:-8545}"
    
    # Encode input: privateKeyId + publicKey
    local input_data="${private_key_id}${public_key#0x}"
    
    echo "Performing ECDH key exchange..." >&2
    
    # Send transaction
    local tx_hash=$(send_to_precompiled_contract "$SGX_ECDH" "$input_data" "$from_address" "$rpc_port")
    
    if [ -z "$tx_hash" ] || [ "$tx_hash" = "null" ]; then
        echo "Failed to send transaction" >&2
        return 1
    fi
    
    # Wait for transaction
    local receipt=$(wait_for_transaction "$tx_hash" "$rpc_port")
    
    # Extract shared secret from logs
    local shared_secret=$(echo "$receipt" | jq -r '.logs[0].data // empty')
    
    echo "$shared_secret"
}
