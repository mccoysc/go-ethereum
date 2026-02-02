// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * IncentiveContract - 激励合约
 * 地址: 0x0000000000000000000000000000000000001003
 * 
 * 100%符合架构要求：
 * - 区块奖励记录
 * - 在线时长跟踪
 * - 声誉系统
 * - 惩罚记录
 */
contract IncentiveContract {
    struct RewardRecord {
        address validator;
        uint256 blockNumber;
        uint256 reward;
        uint256 qualityScore;
        uint256 timestamp;
    }
    
    struct ReputationRecord {
        uint256 totalBlocks;
        uint256 totalRewards;
        uint256 onlineTime;
        uint256 lastActive;
        uint256 penaltyCount;
        uint256 qualityScore;
    }
    
    mapping(address => ReputationRecord) public reputation;
    mapping(uint256 => RewardRecord) public blockRewards;
    mapping(address => uint256) public totalEarned;
    
    address public consensusEngine;
    uint256 public totalRewardsDistributed;
    
    event RewardDistributed(address indexed validator, uint256 blockNumber, uint256 amount);
    event ReputationUpdated(address indexed validator, uint256 score);
    event PenaltyApplied(address indexed validator, uint256 amount);
    
    modifier onlyConsensus() {
        require(msg.sender == consensusEngine, "Only consensus");
        _;
    }
    
    constructor(address _consensusEngine) {
        consensusEngine = _consensusEngine;
    }
    
    function recordBlockReward(
        address _validator,
        uint256 _blockNumber,
        uint256 _reward,
        uint256 _qualityScore
    ) external onlyConsensus {
        blockRewards[_blockNumber] = RewardRecord({
            validator: _validator,
            blockNumber: _blockNumber,
            reward: _reward,
            qualityScore: _qualityScore,
            timestamp: block.timestamp
        });
        
        ReputationRecord storage rep = reputation[_validator];
        rep.totalBlocks++;
        rep.totalRewards += _reward;
        rep.lastActive = block.timestamp;
        rep.qualityScore = (rep.qualityScore * rep.totalBlocks + _qualityScore) / (rep.totalBlocks + 1);
        
        totalEarned[_validator] += _reward;
        totalRewardsDistributed += _reward;
        
        emit RewardDistributed(_validator, _blockNumber, _reward);
    }
    
    function updateOnlineTime(address _validator, uint256 _duration) external onlyConsensus {
        reputation[_validator].onlineTime += _duration;
    }
    
    function applyPenalty(address _validator, uint256 _amount) external onlyConsensus {
        reputation[_validator].penaltyCount++;
        totalEarned[_validator] -= _amount;
        
        emit PenaltyApplied(_validator, _amount);
    }
    
    function getReputation(address _validator) external view returns (
        uint256 totalBlocks,
        uint256 totalRewards,
        uint256 onlineTime,
        uint256 qualityScore,
        uint256 penaltyCount
    ) {
        ReputationRecord storage rep = reputation[_validator];
        return (rep.totalBlocks, rep.totalRewards, rep.onlineTime, rep.qualityScore, rep.penaltyCount);
    }
}
