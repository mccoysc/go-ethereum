#!/bin/bash
# Contract interaction utilities for E2E tests

# Call a precompiled contract
call_precompiled_contract() {
    local contract_address="$1"
    local input_data="$2"
    local from_address="${3:-0x0000000000000000000000000000000000000000}"
    local rpc_port="${4:-8545}"
    
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":[{\"from\":\"$from_address\",\"to\":\"$contract_address\",\"data\":\"$input_data\"},\"latest\"],\"id\":1}" \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result'
}

# Send a transaction to a precompiled contract
send_to_precompiled_contract() {
    local contract_address="$1"
    local input_data="$2"
    local from_address="$3"
    local rpc_port="${4:-8545}"
    local gas="${5:-1000000}"
    
    # First, unlock the account (for testing)
    unlock_account "$from_address" "" "$rpc_port"
    
    # Send transaction
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendTransaction\",\"params\":[{\"from\":\"$from_address\",\"to\":\"$contract_address\",\"data\":\"$input_data\",\"gas\":\"0x$(printf '%x' $gas)\"}],\"id\":1}" \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result'
}

# Unlock an account
unlock_account() {
    local address="$1"
    local password="${2:-}"
    local rpc_port="${3:-8545}"
    local duration="${4:-300}"
    
    curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"personal_unlockAccount\",\"params\":[\"$address\",\"$password\",$duration],\"id\":1}" \
        "http://127.0.0.1:$rpc_port" > /dev/null
}

# Get transaction receipt
get_transaction_receipt() {
    local tx_hash="$1"
    local rpc_port="${2:-8545}"
    
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx_hash\"],\"id\":1}" \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result'
}

# Check if transaction succeeded
is_transaction_success() {
    local tx_hash="$1"
    local rpc_port="${2:-8545}"
    
    local receipt=$(get_transaction_receipt "$tx_hash" "$rpc_port")
    local status=$(echo "$receipt" | jq -r '.status')
    
    if [ "$status" = "0x1" ]; then
        return 0
    else
        return 1
    fi
}

# Get contract code
get_contract_code() {
    local address="$1"
    local rpc_port="${2:-8545}"
    
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$address\",\"latest\"],\"id\":1}" \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result'
}

# Encode function call (simple version for testing)
encode_function_call() {
    local function_signature="$1"
    shift
    local args=("$@")
    
    # Calculate function selector (first 4 bytes of keccak256 hash)
    # For testing, we'll use a simple encoding
    # In production, this should use proper ABI encoding
    
    # Return just the function selector for now
    echo "0x$(echo -n "$function_signature" | sha256sum | cut -c1-8)"
}

# Decode hex to ASCII
hex_to_ascii() {
    local hex_string="$1"
    # Remove 0x prefix if present
    hex_string="${hex_string#0x}"
    echo "$hex_string" | xxd -r -p
}

# Pad hex to 32 bytes (64 hex chars)
pad_hex() {
    local hex_value="$1"
    hex_value="${hex_value#0x}"
    printf "0x%064s" "$hex_value" | tr ' ' '0'
}

# Convert number to hex
num_to_hex() {
    local num="$1"
    printf "0x%x" "$num"
}

# Convert hex to number
hex_to_num() {
    local hex="$1"
    hex="${hex#0x}"
    echo $((16#$hex))
}

# Alias for call_precompiled_contract
call_contract() {
    call_precompiled_contract "$@"
}

# Make JSON-RPC call
call_rpc() {
    local method="$1"
    local params="$2"
    local rpc_port="${3:-8545}"
    
    curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}" \
        "http://127.0.0.1:$rpc_port"
}
