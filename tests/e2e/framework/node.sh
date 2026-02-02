#!/bin/bash
# Node management functions for E2E tests

# Get the project root directory
get_project_root() {
    echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../" && pwd)"
}

# Get the geth binary path
get_geth_binary() {
    local project_root=$(get_project_root)
    echo "${project_root}/build/bin/geth"
}

# Create a temporary directory for test node data
create_test_datadir() {
    local test_name="$1"
    local datadir="/tmp/xchain-e2e-${test_name}-$$"
    mkdir -p "$datadir"
    echo "$datadir"
}

# Alias for create_test_datadir for compatibility
setup_test_dir() {
    create_test_datadir "$@"
}

# Initialize a test node with custom genesis
init_test_node() {
    local datadir="$1"
    local genesis_file="$2"
    local geth=$(get_geth_binary)
    
    echo "Initializing test node in $datadir..."
    $geth --datadir "$datadir" init "$genesis_file" 2>&1 | grep -v "WARN"
    
    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        echo "Node initialized successfully"
        return 0
    else
        echo "Failed to initialize node"
        return 1
    fi
}

# Start a test node
start_test_node() {
    local datadir="$1"
    local port="${2:-30303}"
    local rpc_port="${3:-8545}"
    local geth=$(get_geth_binary)
    
    echo "Starting test node with PoA-SGX consensus on port $port, RPC port $rpc_port..."
    
    # Setup test environment before starting node
    setup_test_filesystem
    
    # Ensure all environment variables are set for SGX consensus
    # These MUST be exported before starting geth
    source "$(dirname "${BASH_SOURCE[0]}")/test_env.sh"
    export XCHAIN_SGX_MODE=mock
    export GRAMINE_VERSION=test
    
    # Create a miner account if not exists
    local keystore_dir="$datadir/keystore"
    local miner_account=""
    if [ ! -d "$keystore_dir" ] || [ -z "$(ls -A $keystore_dir 2>/dev/null)" ]; then
        echo "Creating miner account..."
        echo "password" > "$datadir/password.txt"
        miner_account=$(echo "password" | $geth --datadir "$datadir" account new --password /dev/stdin 2>&1 | grep "Public address" | awk '{print $NF}')
        echo "Miner account created: $miner_account"
    else
        # Get existing account
        local keyfile=$(ls "$keystore_dir" | head -1)
        if [ -n "$keyfile" ]; then
            miner_account="0x$(echo $keyfile | grep -o '[0-9a-fA-F]\{40\}')"
            echo "Using existing account: $miner_account"
            echo "password" > "$datadir/password.txt"
        fi
    fi
    
    # Start geth in background with PoA-SGX consensus and HTTP RPC enabled
    # Unlock the miner account for block signing
    # Use nohup and redirect to ensure proper background execution
    nohup $geth --datadir "$datadir" \
        --networkid 762385986 \
        --port "$port" \
        --http \
        --http.addr "127.0.0.1" \
        --http.port "$rpc_port" \
        --http.api "eth,net,web3,personal,admin,debug,txpool,sgx,miner" \
        --http.corsdomain "*" \
        --nodiscover \
        --maxpeers 0 \
        --allow-insecure-unlock \
        --unlock "$miner_account" \
        --password "$datadir/password.txt" \
        --mine \
        --miner.etherbase "$miner_account" \
        --verbosity 3 \
        >> "$datadir/geth.log" 2>&1 &
    
    local pid=$!
    echo $pid > "$datadir/geth.pid"
    
    # Wait for node to start
    local max_attempts=60
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s -X POST -H "Content-Type: application/json" \
            --data '{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}' \
            "http://127.0.0.1:$rpc_port" > /dev/null 2>&1; then
            echo "Node started successfully (PID: $pid)"
            return 0
        fi
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo "Failed to start node after $max_attempts seconds"
    echo "Check logs at: $datadir/geth.log"
    if [ -f "$datadir/geth.log" ]; then
        echo "=== Last 20 lines of log ==="
        tail -20 "$datadir/geth.log"
    fi
    return 1
}

# Stop a test node
stop_test_node() {
    local datadir="$1"
    
    if [ -f "$datadir/geth.pid" ]; then
        local pid=$(cat "$datadir/geth.pid")
        echo "Stopping node (PID: $pid)..."
        kill $pid 2>/dev/null
        
        # Wait for process to stop
        local max_attempts=10
        local attempt=0
        while [ $attempt -lt $max_attempts ]; do
            if ! ps -p $pid > /dev/null 2>&1; then
                echo "Node stopped successfully"
                rm "$datadir/geth.pid"
                return 0
            fi
            sleep 1
            attempt=$((attempt + 1))
        done
        
        echo "Force killing node..."
        kill -9 $pid 2>/dev/null
        rm "$datadir/geth.pid"
    fi
}

# Clean up test node data
cleanup_test_node() {
    local datadir="$1"
    
    echo "Cleaning up test data: $datadir"
    stop_test_node "$datadir"
    rm -rf "$datadir"
}

# Create a new account
create_account() {
    local datadir="$1"
    local password="${2:-}"
    local geth=$(get_geth_binary)
    
    if [ -z "$password" ]; then
        # Create account without password
        echo "" | $geth --datadir "$datadir" account new 2>&1 | grep "Public address" | cut -d' ' -f4
    else
        # Create account with password
        echo "$password" | $geth --datadir "$datadir" account new --password /dev/stdin 2>&1 | grep "Public address" | cut -d' ' -f4
    fi
}

# Get node info via RPC
get_node_info() {
    local rpc_port="${1:-8545}"
    
    curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
        "http://127.0.0.1:$rpc_port"
}

# Get current block number
get_block_number() {
    local rpc_port="${1:-8545}"
    
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result' | sed 's/0x//' | awk '{print "ibase=16; " toupper($0)}' | bc
}

# Get account balance
get_balance() {
    local address="$1"
    local rpc_port="${2:-8545}"
    
    local result=$(curl -s -X POST -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$address\",\"latest\"],\"id\":1}" \
        "http://127.0.0.1:$rpc_port")
    
    echo "$result" | jq -r '.result'
}

# Wait for transaction to be mined
wait_for_transaction() {
    local tx_hash="$1"
    local rpc_port="${2:-8545}"
    local max_attempts="${3:-30}"
    
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        local result=$(curl -s -X POST -H "Content-Type: application/json" \
            --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionReceipt\",\"params\":[\"$tx_hash\"],\"id\":1}" \
            "http://127.0.0.1:$rpc_port")
        
        local receipt=$(echo "$result" | jq -r '.result')
        
        if [ "$receipt" != "null" ]; then
            echo "$receipt"
            return 0
        fi
        
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo "Transaction not mined after $max_attempts seconds"
    return 1
}

# Cleanup test directory and node
cleanup_test() {
    local test_dir="$1"
    cleanup_test_node "$test_dir"
    cleanup_test_filesystem
}
