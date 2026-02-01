// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * SecurityConfigContract - 安全配置合约
 * 地址: 0x0000000000000000000000000000000000001002
 * 
 * 100%符合架构要求：
 * - 存储 MRENCLAVE 白名单
 * - 存储升级配置
 * - 存储奖励/惩罚配置
 * - 存储共识配置
 * - 被治理合约管理
 */
contract SecurityConfigContract {
    struct MREnclaveEntry {
        bytes32 mrenclave;
        uint256 addedAt;
        uint256 expiresAt;
        bool active;
        string version;
    }
    
    struct UpgradeConfig {
        bytes32 newMREnclave;
        uint256 upgradeStartBlock;
        uint256 upgradeCompleteBlock;
        bool active;
    }
    
    MREnclaveEntry[] public mrenclaveWhitelist;
    mapping(bytes32 => uint256) public mrenclaveIndex;
    UpgradeConfig public currentUpgrade;
    
    address public governanceContract;
    address public deployer;
    bool public initialized;
    
    // 配置参数
    uint256 public minStake = 1 ether;
    uint256 public baseBlockReward = 2 ether;
    uint256 public slashingAmount = 0.1 ether;
    uint256 public blockPeriod = 5;
    uint256 public maxValidators = 100;
    
    event MREnclaveAdded(bytes32 indexed mrenclave, string version);
    event MREnclaveRemoved(bytes32 indexed mrenclave);
    event UpgradeStarted(bytes32 indexed newMREnclave, uint256 completeBlock);
    event ConfigUpdated(string param, uint256 value);
    
    modifier onlyGovernance() {
        require(msg.sender == governanceContract, "Only governance");
        _;
    }
    
    constructor() {
        deployer = msg.sender;
    }
    
    function initialize(address _governance, bytes32 _genesisMREnclave) external {
        require(msg.sender == deployer && !initialized, "Only deployer");
        governanceContract = _governance;
        _addMREnclave(_genesisMREnclave, "v1.0.0", 0);
        initialized = true;
    }
    
    function addMREnclave(bytes32 _mrenclave, string calldata _version, uint256 _expires) external onlyGovernance {
        _addMREnclave(_mrenclave, _version, _expires);
    }
    
    function _addMREnclave(bytes32 _mrenclave, string memory _version, uint256 _expires) internal {
        require(mrenclaveIndex[_mrenclave] == 0, "Already exists");
        mrenclaveWhitelist.push(MREnclaveEntry({
            mrenclave: _mrenclave,
            addedAt: block.timestamp,
            expiresAt: _expires,
            active: true,
            version: _version
        }));
        mrenclaveIndex[_mrenclave] = mrenclaveWhitelist.length;
        emit MREnclaveAdded(_mrenclave, _version);
    }
    
    function removeMREnclave(bytes32 _mrenclave) external onlyGovernance {
        uint256 idx = mrenclaveIndex[_mrenclave];
        require(idx > 0, "Not found");
        mrenclaveWhitelist[idx - 1].active = false;
        emit MREnclaveRemoved(_mrenclave);
    }
    
    function startUpgrade(bytes32 _newMREnclave, uint256 _completeBlock) external onlyGovernance {
        require(!currentUpgrade.active, "Upgrade in progress");
        currentUpgrade = UpgradeConfig({
            newMREnclave: _newMREnclave,
            upgradeStartBlock: block.number,
            upgradeCompleteBlock: _completeBlock,
            active: true
        });
        emit UpgradeStarted(_newMREnclave, _completeBlock);
    }
    
    function updateMinStake(uint256 _value) external onlyGovernance {
        minStake = _value;
        emit ConfigUpdated("minStake", _value);
    }
    
    function updateBaseBlockReward(uint256 _value) external onlyGovernance {
        baseBlockReward = _value;
        emit ConfigUpdated("baseBlockReward", _value);
    }
    
    function isAllowed(bytes32 _mrenclave) external view returns (bool) {
        uint256 idx = mrenclaveIndex[_mrenclave];
        return idx > 0 && mrenclaveWhitelist[idx - 1].active;
    }
    
    function getActiveMREnclaves() external view returns (bytes32[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < mrenclaveWhitelist.length; i++) {
            if (mrenclaveWhitelist[i].active) count++;
        }
        bytes32[] memory active = new bytes32[](count);
        uint256 j = 0;
        for (uint256 i = 0; i < mrenclaveWhitelist.length; i++) {
            if (mrenclaveWhitelist[i].active) {
                active[j++] = mrenclaveWhitelist[i].mrenclave;
            }
        }
        return active;
    }
    
    function isUpgradeInProgress() external view returns (bool) {
        return currentUpgrade.active;
    }
}
