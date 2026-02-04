// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title SGXCryptoTest
 * @dev Test contract for SGX cryptographic precompiled contracts
 */
contract SGXCryptoTest {
    
    // Precompiled contract addresses
    address constant SGX_KEY_CREATE = address(0x8000);
    address constant SGX_KEY_GET_PUBLIC = address(0x8001);
    address constant SGX_SIGN = address(0x8002);
    address constant SGX_VERIFY = address(0x8003);
    address constant SGX_ECDH = address(0x8004);
    address constant SGX_RANDOM = address(0x8005);
    address constant SGX_ENCRYPT = address(0x8006);
    address constant SGX_DECRYPT = address(0x8007);
    address constant SGX_KEY_DERIVE = address(0x8008);
    
    // Events for test results
    event TestResult(string testName, bool success, string message);
    
    // Test: Create key pair
    function testKeyCreate() public returns (bytes32) {
        (bool success, bytes memory result) = SGX_KEY_CREATE.call("");
        require(success, "Key creation failed");
        bytes32 keyId = abi.decode(result, (bytes32));
        emit TestResult("testKeyCreate", true, "Key created successfully");
        return keyId;
    }
    
    // Test: Comprehensive workflow
    function testCompleteWorkflow() public returns (bool) {
        // 1. Create key
        bytes32 keyId = testKeyCreate();
        
        // 2. Get public key
        (bool success, bytes memory result) = SGX_KEY_GET_PUBLIC.call(abi.encode(keyId));
        require(success, "Get public key failed");
        bytes memory publicKey = abi.decode(result, (bytes));
        
        // 3. Sign data
        bytes memory testData = "Hello SGX!";
        (success, result) = SGX_SIGN.call(abi.encode(keyId, testData));
        require(success, "Signing failed");
        bytes memory signature = abi.decode(result, (bytes));
        
        // 4. Verify signature
        (success, result) = SGX_VERIFY.call(abi.encode(publicKey, testData, signature));
        require(success, "Verification call failed");
        bool valid = abi.decode(result, (bool));
        require(valid, "Signature verification failed");
        
        // 5. Encrypt/Decrypt
        (success, result) = SGX_ENCRYPT.call(abi.encode(keyId, testData));
        require(success, "Encryption failed");
        bytes memory encrypted = abi.decode(result, (bytes));
        
        (success, result) = SGX_DECRYPT.call(abi.encode(keyId, encrypted));
        require(success, "Decryption failed");
        bytes memory decrypted = abi.decode(result, (bytes));
        require(keccak256(testData) == keccak256(decrypted), "Encryption round-trip failed");
        
        // 6. Generate random
        (success, result) = SGX_RANDOM.call(abi.encode(uint256(32)));
        require(success, "Random generation failed");
        bytes memory randomData = abi.decode(result, (bytes));
        require(randomData.length == 32, "Random data length incorrect");
        
        emit TestResult("testCompleteWorkflow", true, "All tests passed");
        return true;
    }
}
