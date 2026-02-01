// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * @title IncentiveContract
 * @dev 激励合约 - 管理挖矿奖励和惩罚
 */
contract IncentiveContract {
    struct MinerStats {
        uint256 blocksProduced;
        uint256 totalRewards;
        uint256 lastBlockTime;
        uint256 qualityScore;
        bool active;
    }

    mapping(address => MinerStats) public miners;
    mapping(uint256 => address) public blockProducers;
    
    uint256 public baseReward = 2 ether;
    uint256 public totalBlocksProduced;
    address public owner;

    event BlockRewarded(address indexed miner, uint256 blockNumber, uint256 reward);
    event QualityScoreUpdated(address indexed miner, uint256 score);

    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    // 记录区块生产
    function recordBlock(address _miner, uint256 _blockNumber, uint256 _qualityScore) external onlyOwner {
        miners[_miner].blocksProduced++;
        miners[_miner].lastBlockTime = block.timestamp;
        miners[_miner].qualityScore = _qualityScore;
        miners[_miner].active = true;
        
        blockProducers[_blockNumber] = _miner;
        totalBlocksProduced++;

        emit QualityScoreUpdated(_miner, _qualityScore);
    }

    // 分发奖励
    function distributeReward(address _miner, uint256 _blockNumber) external payable onlyOwner {
        uint256 reward = calculateReward(_miner);
        miners[_miner].totalRewards += reward;
        
        payable(_miner).transfer(reward);
        emit BlockRewarded(_miner, _blockNumber, reward);
    }

    // 计算奖励
    function calculateReward(address _miner) public view returns (uint256) {
        uint256 qualityBonus = (baseReward * miners[_miner].qualityScore) / 10000;
        return baseReward + qualityBonus;
    }

    // 查询矿工统计
    function getMinerStats(address _miner) external view returns (MinerStats memory) {
        return miners[_miner];
    }

    // 更新基础奖励
    function updateBaseReward(uint256 _newReward) external onlyOwner {
        baseReward = _newReward;
    }

    // 接收以太币
    receive() external payable {}
}
