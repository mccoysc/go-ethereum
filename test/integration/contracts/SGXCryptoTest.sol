// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * 测试合约：调用 SGX 预编译密码学接口
 */
contract SGXCryptoTest {
    // SGX 预编译合约地址
    address constant SGX_KEY_CREATE = address(0x8000);
    address constant SGX_KEY_GET_PUBLIC = address(0x8001);
    address constant SGX_SIGN = address(0x8002);
    address constant SGX_VERIFY = address(0x8003);
    address constant SGX_ECDH = address(0x8004);
    address constant SGX_RANDOM = address(0x8005);
    address constant SGX_ENCRYPT = address(0x8006);
    address constant SGX_DECRYPT = address(0x8007);
    address constant SGX_KEY_DERIVE = address(0x8008);
    
    event KeyCreated(bytes32 keyId);
    event SignatureCreated(bytes signature);
    event RandomGenerated(bytes randomData);
    event EncryptionDone(bytes ciphertext);
    
    // 存储测试结果
    mapping(string => bool) public testResults;
    mapping(string => bytes) public testData;
    
    /**
     * 测试密钥创建
     */
    function testKeyCreate() public returns (bool) {
        // 调用 SGX_KEY_CREATE (0x8000)
        bytes memory input = abi.encodePacked(uint8(1)); // key type = 1
        
        (bool success, bytes memory result) = SGX_KEY_CREATE.call(input);
        
        if (success && result.length > 0) {
            testResults["keyCreate"] = true;
            testData["keyId"] = result;
            emit KeyCreated(bytes32(result));
            return true;
        }
        return false;
    }
    
    /**
     * 测试获取公钥
     */
    function testGetPublicKey(bytes32 keyId) public returns (bool) {
        bytes memory input = abi.encodePacked(keyId);
        
        (bool success, bytes memory result) = SGX_KEY_GET_PUBLIC.call(input);
        
        if (success && result.length > 0) {
            testResults["getPublicKey"] = true;
            testData["publicKey"] = result;
            return true;
        }
        return false;
    }
    
    /**
     * 测试签名
     */
    function testSign(bytes32 keyId, bytes32 message) public returns (bool) {
        bytes memory input = abi.encodePacked(keyId, message);
        
        (bool success, bytes memory result) = SGX_SIGN.call(input);
        
        if (success && result.length > 0) {
            testResults["sign"] = true;
            testData["signature"] = result;
            emit SignatureCreated(result);
            return true;
        }
        return false;
    }
    
    /**
     * 测试验证签名
     */
    function testVerify(bytes memory publicKey, bytes32 message, bytes memory signature) public returns (bool) {
        bytes memory input = abi.encodePacked(publicKey, message, signature);
        
        (bool success, bytes memory result) = SGX_VERIFY.call(input);
        
        if (success && result.length > 0) {
            bool verified = abi.decode(result, (bool));
            testResults["verify"] = verified;
            return verified;
        }
        return false;
    }
    
    /**
     * 测试随机数生成
     */
    function testRandom(uint256 length) public returns (bool) {
        bytes memory input = abi.encodePacked(uint32(length));
        
        (bool success, bytes memory result) = SGX_RANDOM.call(input);
        
        if (success && result.length > 0) {
            testResults["random"] = true;
            testData["random"] = result;
            emit RandomGenerated(result);
            return true;
        }
        return false;
    }
    
    /**
     * 测试加密
     */
    function testEncrypt(bytes32 keyId, bytes memory plaintext) public returns (bool) {
        bytes memory input = abi.encodePacked(keyId, plaintext);
        
        (bool success, bytes memory result) = SGX_ENCRYPT.call(input);
        
        if (success && result.length > 0) {
            testResults["encrypt"] = true;
            testData["ciphertext"] = result;
            emit EncryptionDone(result);
            return true;
        }
        return false;
    }
    
    /**
     * 测试解密
     */
    function testDecrypt(bytes32 keyId, bytes memory ciphertext) public returns (bool) {
        bytes memory input = abi.encodePacked(keyId, ciphertext);
        
        (bool success, bytes memory result) = SGX_DECRYPT.call(input);
        
        if (success && result.length > 0) {
            testResults["decrypt"] = true;
            testData["decrypted"] = result;
            return true;
        }
        return false;
    }
    
    /**
     * 测试密钥派生
     */
    function testKeyDerive(bytes32 masterKey, bytes32 salt) public returns (bool) {
        bytes memory input = abi.encodePacked(masterKey, salt);
        
        (bool success, bytes memory result) = SGX_KEY_DERIVE.call(input);
        
        if (success && result.length > 0) {
            testResults["keyDerive"] = true;
            testData["derivedKey"] = result;
            return true;
        }
        return false;
    }
    
    /**
     * 完整的密码学流程测试
     */
    function runFullCryptoTest() public returns (bool) {
        // 1. 创建密钥
        if (!testKeyCreate()) return false;
        bytes32 keyId = bytes32(testData["keyId"]);
        
        // 2. 获取公钥
        if (!testGetPublicKey(keyId)) return false;
        
        // 3. 生成随机数
        if (!testRandom(32)) return false;
        
        // 4. 签名和验证
        bytes32 message = keccak256("Test message");
        if (!testSign(keyId, message)) return false;
        if (!testVerify(testData["publicKey"], message, testData["signature"])) return false;
        
        // 5. 加密和解密
        bytes memory plaintext = "Hello SGX!";
        if (!testEncrypt(keyId, plaintext)) return false;
        if (!testDecrypt(keyId, testData["ciphertext"])) return false;
        
        // 6. 密钥派生
        if (!testKeyDerive(keyId, keccak256("salt"))) return false;
        
        return true;
    }
    
    /**
     * 获取测试结果
     */
    function getTestResult(string memory testName) public view returns (bool) {
        return testResults[testName];
    }
    
    /**
     * 获取所有测试状态
     */
    function getAllTestResults() public view returns (
        bool keyCreate,
        bool getPublicKey,
        bool sign,
        bool verify,
        bool random,
        bool encrypt,
        bool decrypt,
        bool keyDerive
    ) {
        return (
            testResults["keyCreate"],
            testResults["getPublicKey"],
            testResults["sign"],
            testResults["verify"],
            testResults["random"],
            testResults["encrypt"],
            testResults["decrypt"],
            testResults["keyDerive"]
        );
    }
}
