// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title CryptoTestContract
 * @dev 测试合约 - 调用所有SGX密码学预编译接口
 * 
 * 预编译合约地址:
 * 0x8000 - SGX_KEY_CREATE
 * 0x8001 - SGX_KEY_GET_PUBLIC
 * 0x8002 - SGX_SIGN
 * 0x8003 - SGX_VERIFY
 * 0x8004 - SGX_ECDH
 * 0x8005 - SGX_RANDOM
 * 0x8006 - SGX_ENCRYPT
 * 0x8007 - SGX_DECRYPT
 * 0x8008 - SGX_KEY_DERIVE
 */
contract CryptoTestContract {
    
    // 事件用于记录测试结果
    event TestResult(string testName, bool success, bytes result);
    event KeyCreated(bytes32 keyId);
    event SignatureGenerated(bytes signature);
    event EncryptionResult(bytes ciphertext);
    event RandomGenerated(bytes randomData);
    
    // 存储密钥ID
    bytes32 public lastKeyId;
    bytes public lastPublicKey;
    bytes public lastSignature;
    bytes public lastCiphertext;
    bytes public lastPlaintext;
    bytes public lastRandomData;
    
    /**
     * @dev 测试SGX_KEY_CREATE (0x8000)
     * 在SGX Enclave内创建密钥对
     */
    function testKeyCreate() public returns (bytes32 keyId) {
        address precompile = address(0x8000);
        
        // 调用预编译合约创建密钥
        (bool success, bytes memory result) = precompile.call("");
        require(success, "KEY_CREATE failed");
        require(result.length == 32, "Invalid key ID length");
        
        keyId = bytes32(result);
        lastKeyId = keyId;
        
        emit KeyCreated(keyId);
        emit TestResult("KEY_CREATE", true, result);
        
        return keyId;
    }
    
    /**
     * @dev 测试SGX_KEY_GET_PUBLIC (0x8001)
     * 获取密钥的公钥部分
     */
    function testGetPublicKey(bytes32 keyId) public returns (bytes memory) {
        address precompile = address(0x8001);
        
        // 调用预编译合约获取公钥
        (bool success, bytes memory publicKey) = precompile.call(abi.encode(keyId));
        require(success, "GET_PUBLIC failed");
        require(publicKey.length > 0, "Empty public key");
        
        lastPublicKey = publicKey;
        
        emit TestResult("GET_PUBLIC", true, publicKey);
        
        return publicKey;
    }
    
    /**
     * @dev 测试SGX_SIGN (0x8002)
     * 使用SGX密钥签名数据
     */
    function testSign(bytes32 keyId, bytes memory data) public returns (bytes memory) {
        address precompile = address(0x8002);
        
        // 调用预编译合约签名
        (bool success, bytes memory signature) = precompile.call(abi.encode(keyId, data));
        require(success, "SIGN failed");
        require(signature.length > 0, "Empty signature");
        
        lastSignature = signature;
        
        emit SignatureGenerated(signature);
        emit TestResult("SIGN", true, signature);
        
        return signature;
    }
    
    /**
     * @dev 测试SGX_VERIFY (0x8003)
     * 验证签名
     */
    function testVerify(bytes memory publicKey, bytes memory data, bytes memory signature) public returns (bool) {
        address precompile = address(0x8003);
        
        // 调用预编译合约验证签名
        (bool success, bytes memory result) = precompile.call(abi.encode(publicKey, data, signature));
        require(success, "VERIFY call failed");
        
        bool isValid = result.length > 0 && result[0] == 0x01;
        
        emit TestResult("VERIFY", isValid, result);
        
        return isValid;
    }
    
    /**
     * @dev 测试SGX_ECDH (0x8004)
     * 密钥交换
     */
    function testECDH(bytes32 myKeyId, bytes memory theirPublicKey) public returns (bytes memory) {
        address precompile = address(0x8004);
        
        // 调用预编译合约进行密钥交换
        (bool success, bytes memory sharedSecret) = precompile.call(
            abi.encode(myKeyId, theirPublicKey)
        );
        require(success, "ECDH failed");
        require(sharedSecret.length > 0, "Empty shared secret");
        
        emit TestResult("ECDH", true, sharedSecret);
        
        return sharedSecret;
    }
    
    /**
     * @dev 测试SGX_RANDOM (0x8005)
     * 生成随机数
     */
    function testRandom(uint256 length) public returns (bytes memory) {
        address precompile = address(0x8005);
        
        // 调用预编译合约生成随机数
        (bool success, bytes memory randomData) = precompile.call(abi.encode(length));
        require(success, "RANDOM failed");
        require(randomData.length == length, "Invalid random data length");
        
        lastRandomData = randomData;
        
        emit RandomGenerated(randomData);
        emit TestResult("RANDOM", true, randomData);
        
        return randomData;
    }
    
    /**
     * @dev 测试SGX_ENCRYPT (0x8006)
     * 加密数据
     */
    function testEncrypt(bytes32 keyId, bytes memory plaintext) public returns (bytes memory) {
        address precompile = address(0x8006);
        
        // 调用预编译合约加密
        (bool success, bytes memory ciphertext) = precompile.call(
            abi.encode(keyId, plaintext)
        );
        require(success, "ENCRYPT failed");
        require(ciphertext.length > 0, "Empty ciphertext");
        
        lastCiphertext = ciphertext;
        
        emit EncryptionResult(ciphertext);
        emit TestResult("ENCRYPT", true, ciphertext);
        
        return ciphertext;
    }
    
    /**
     * @dev 测试SGX_DECRYPT (0x8007)
     * 解密数据
     */
    function testDecrypt(bytes32 keyId, bytes memory ciphertext) public returns (bytes memory) {
        address precompile = address(0x8007);
        
        // 调用预编译合约解密
        (bool success, bytes memory plaintext) = precompile.call(
            abi.encode(keyId, ciphertext)
        );
        require(success, "DECRYPT failed");
        require(plaintext.length > 0, "Empty plaintext");
        
        lastPlaintext = plaintext;
        
        emit TestResult("DECRYPT", true, plaintext);
        
        return plaintext;
    }
    
    /**
     * @dev 测试SGX_KEY_DERIVE (0x8008)
     * 从主密钥派生子密钥
     */
    function testKeyDerive(bytes32 masterKeyId, bytes memory context) public returns (bytes32) {
        address precompile = address(0x8008);
        
        // 调用预编译合约派生密钥
        (bool success, bytes memory result) = precompile.call(
            abi.encode(masterKeyId, context)
        );
        require(success, "KEY_DERIVE failed");
        require(result.length == 32, "Invalid derived key ID length");
        
        bytes32 derivedKeyId = bytes32(result);
        
        emit TestResult("KEY_DERIVE", true, result);
        
        return derivedKeyId;
    }
    
    /**
     * @dev 完整的加密解密测试流程
     */
    function testFullEncryptionCycle(string memory message) public returns (bool) {
        // 1. 创建密钥
        bytes32 keyId = testKeyCreate();
        
        // 2. 加密消息
        bytes memory plaintext = bytes(message);
        bytes memory ciphertext = testEncrypt(keyId, plaintext);
        
        // 3. 解密消息
        bytes memory decrypted = testDecrypt(keyId, ciphertext);
        
        // 4. 验证解密结果
        require(keccak256(plaintext) == keccak256(decrypted), "Decryption mismatch");
        
        emit TestResult("FULL_ENCRYPTION_CYCLE", true, decrypted);
        
        return true;
    }
    
    /**
     * @dev 完整的签名验证测试流程
     */
    function testFullSignatureCycle(string memory message) public returns (bool) {
        // 1. 创建密钥
        bytes32 keyId = testKeyCreate();
        
        // 2. 获取公钥
        bytes memory publicKey = testGetPublicKey(keyId);
        
        // 3. 签名消息
        bytes memory data = bytes(message);
        bytes memory signature = testSign(keyId, data);
        
        // 4. 验证签名
        bool isValid = testVerify(publicKey, data, signature);
        require(isValid, "Signature verification failed");
        
        emit TestResult("FULL_SIGNATURE_CYCLE", true, abi.encodePacked(isValid));
        
        return true;
    }
    
    /**
     * @dev 测试所有接口
     */
    function testAllInterfaces() public returns (bool) {
        // 测试随机数生成
        testRandom(32);
        
        // 测试完整加密周期
        testFullEncryptionCycle("Hello SGX!");
        
        // 测试完整签名周期
        testFullSignatureCycle("Sign this message");
        
        emit TestResult("ALL_INTERFACES", true, "");
        
        return true;
    }
}
