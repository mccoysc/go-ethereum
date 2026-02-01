// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * @title SecurityConfigContract
 * @dev 安全配置合约 - 管理系统安全参数
 */
contract SecurityConfigContract {
    // 安全配置
    struct SecurityConfig {
        uint256 minBlockInterval;      // 最小出块间隔（秒）
        uint256 maxBlockInterval;      // 最大出块间隔（秒）
        uint256 maxTxPerBlock;         // 单区块最大交易数
        uint256 maxGasPerBlock;        // 单区块最大 Gas
        uint256 minStake;              // 最小质押金额
        uint256 slashingAmount;        // 惩罚金额
        bool encryptionRequired;       // 是否要求加密
        bool sgxRequired;              // 是否要求 SGX
    }

    SecurityConfig public config;
    address public owner;
    mapping(address => bool) public admins;

    event ConfigUpdated(string param, uint256 value);
    event AdminAdded(address indexed admin);
    event AdminRemoved(address indexed admin);

    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner");
        _;
    }

    modifier onlyAdmin() {
        require(admins[msg.sender] || msg.sender == owner, "Only admin");
        _;
    }

    constructor() {
        owner = msg.sender;
        admins[msg.sender] = true;
        
        // 初始化默认配置
        config = SecurityConfig({
            minBlockInterval: 1,
            maxBlockInterval: 60,
            maxTxPerBlock: 1000,
            maxGasPerBlock: 30000000,
            minStake: 1 ether,
            slashingAmount: 0.1 ether,
            encryptionRequired: true,
            sgxRequired: true
        });
    }

    function updateMinBlockInterval(uint256 _value) external onlyAdmin {
        config.minBlockInterval = _value;
        emit ConfigUpdated("minBlockInterval", _value);
    }

    function updateMaxBlockInterval(uint256 _value) external onlyAdmin {
        config.maxBlockInterval = _value;
        emit ConfigUpdated("maxBlockInterval", _value);
    }

    function updateMaxTxPerBlock(uint256 _value) external onlyAdmin {
        config.maxTxPerBlock = _value;
        emit ConfigUpdated("maxTxPerBlock", _value);
    }

    function updateMaxGasPerBlock(uint256 _value) external onlyAdmin {
        config.maxGasPerBlock = _value;
        emit ConfigUpdated("maxGasPerBlock", _value);
    }

    function updateMinStake(uint256 _value) external onlyAdmin {
        config.minStake = _value;
        emit ConfigUpdated("minStake", _value);
    }

    function addAdmin(address _admin) external onlyOwner {
        admins[_admin] = true;
        emit AdminAdded(_admin);
    }

    function removeAdmin(address _admin) external onlyOwner {
        admins[_admin] = false;
        emit AdminRemoved(_admin);
    }

    function getConfig() external view returns (SecurityConfig memory) {
        return config;
    }
}
