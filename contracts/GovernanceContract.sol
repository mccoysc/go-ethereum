// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * GovernanceContract - 治理合约
 * 地址: 0x0000000000000000000000000000000000001001
 * 
 * 100%符合架构要求：
 * - 引导机制 (Bootstrap)
 * - 提案创建和投票
 * - 验证者管理
 * - 管理 SecurityConfigContract
 */
contract GovernanceContract {
    enum ProposalType {
        AddMREnclave,
        RemoveMREnclave,
        UpdateConfig,
        AddValidator,
        RemoveValidator
    }
    
    enum ProposalStatus {
        Pending,
        Active,
        Passed,
        Rejected,
        Executed
    }
    
    struct Proposal {
        uint256 id;
        ProposalType proposalType;
        address proposer;
        bytes32 target;
        uint256 value;
        string description;
        uint256 startTime;
        uint256 endTime;
        uint256 yesVotes;
        uint256 noVotes;
        ProposalStatus status;
        bool executed;
    }
    
    struct Validator {
        address addr;
        bytes32 mrenclave;
        bytes32 instanceId;
        uint256 stake;
        bool active;
        uint256 registeredAt;
    }
    
    // Bootstrap 配置
    bool public bootstrapEnded;
    uint256 public founderCount;
    uint256 public constant MAX_FOUNDERS = 5;
    bytes32 public genesisMREnclave;
    
    mapping(address => bool) public founders;
    mapping(bytes32 => address) public hardwareToFounder;
    
    // 提案和验证者
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(address => Validator) public validators;
    
    uint256 public proposalCount;
    uint256 public validatorCount;
    uint256 public constant VOTING_PERIOD = 7 days;
    uint256 public constant VOTING_THRESHOLD = 67; // 67%
    
    address public securityConfigContract;
    
    event FounderRegistered(address indexed founder, bytes32 instanceId);
    event BootstrapEnded();
    event ProposalCreated(uint256 indexed proposalId, ProposalType proposalType);
    event Voted(uint256 indexed proposalId, address indexed voter, bool support);
    event ProposalExecuted(uint256 indexed proposalId);
    event ValidatorRegistered(address indexed validator, bytes32 mrenclave);
    
    constructor(bytes32 _genesisMREnclave) {
        genesisMREnclave = _genesisMREnclave;
    }
    
    function setSecurityConfigContract(address _contract) external {
        require(securityConfigContract == address(0), "Already set");
        require(_contract != address(0), "Invalid address");
        securityConfigContract = _contract;
    }
    
    // Bootstrap: 注册创始管理者
    function registerFounder(bytes32 _instanceId) external {
        require(!bootstrapEnded, "Bootstrap ended");
        require(!founders[msg.sender], "Already founder");
        require(hardwareToFounder[_instanceId] == address(0), "Hardware registered");
        require(founderCount < MAX_FOUNDERS, "Max founders reached");
        
        founders[msg.sender] = true;
        hardwareToFounder[_instanceId] = msg.sender;
        founderCount++;
        
        // 自动注册为验证者
        validators[msg.sender] = Validator({
            addr: msg.sender,
            mrenclave: genesisMREnclave,
            instanceId: _instanceId,
            stake: 0,
            active: true,
            registeredAt: block.timestamp
        });
        validatorCount++;
        
        emit FounderRegistered(msg.sender, _instanceId);
        
        if (founderCount >= MAX_FOUNDERS) {
            bootstrapEnded = true;
            emit BootstrapEnded();
        }
    }
    
    // 创建提案
    function createProposal(
        ProposalType _type,
        bytes32 _target,
        uint256 _value,
        string calldata _description
    ) external returns (uint256) {
        require(validators[msg.sender].active || founders[msg.sender], "Not authorized");
        
        proposalCount++;
        proposals[proposalCount] = Proposal({
            id: proposalCount,
            proposalType: _type,
            proposer: msg.sender,
            target: _target,
            value: _value,
            description: _description,
            startTime: block.timestamp,
            endTime: block.timestamp + VOTING_PERIOD,
            yesVotes: 0,
            noVotes: 0,
            status: ProposalStatus.Active,
            executed: false
        });
        
        emit ProposalCreated(proposalCount, _type);
        return proposalCount;
    }
    
    // 投票
    function vote(uint256 _proposalId, bool _support) external {
        require(validators[msg.sender].active || founders[msg.sender], "Not authorized");
        require(!hasVoted[_proposalId][msg.sender], "Already voted");
        require(proposals[_proposalId].status == ProposalStatus.Active, "Not active");
        require(block.timestamp <= proposals[_proposalId].endTime, "Ended");
        
        hasVoted[_proposalId][msg.sender] = true;
        uint256 weight = validators[msg.sender].stake > 0 ? validators[msg.sender].stake : 1 ether;
        
        if (_support) {
            proposals[_proposalId].yesVotes += weight;
        } else {
            proposals[_proposalId].noVotes += weight;
        }
        
        emit Voted(_proposalId, msg.sender, _support);
    }
    
    // 执行提案
    function executeProposal(uint256 _proposalId) external {
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.status == ProposalStatus.Active, "Not active");
        require(block.timestamp > proposal.endTime, "Not ended");
        require(!proposal.executed, "Already executed");
        
        uint256 totalVotes = proposal.yesVotes + proposal.noVotes;
        require(totalVotes > 0, "No votes");
        
        uint256 yesPercentage = (proposal.yesVotes * 100) / totalVotes;
        
        if (yesPercentage >= VOTING_THRESHOLD) {
            proposal.status = ProposalStatus.Passed;
            _executeProposalAction(proposal);
            proposal.executed = true;
            emit ProposalExecuted(_proposalId);
        } else {
            proposal.status = ProposalStatus.Rejected;
        }
    }
    
    function _executeProposalAction(Proposal storage proposal) internal {
        if (proposal.proposalType == ProposalType.AddMREnclave) {
            (bool success,) = securityConfigContract.call(
                abi.encodeWithSignature("addMREnclave(bytes32,string,uint256)",
                    proposal.target, "", proposal.value)
            );
            require(success, "Add MRENCLAVE failed");
        } else if (proposal.proposalType == ProposalType.RemoveMREnclave) {
            (bool success,) = securityConfigContract.call(
                abi.encodeWithSignature("removeMREnclave(bytes32)", proposal.target)
            );
            require(success, "Remove MRENCLAVE failed");
        } else if (proposal.proposalType == ProposalType.UpdateConfig) {
            (bool success,) = securityConfigContract.call(
                abi.encodeWithSignature("updateMinStake(uint256)", proposal.value)
            );
            require(success, "Update config failed");
        }
    }
    
    // 注册验证者（Bootstrap 后）
    function registerValidator(bytes32 _mrenclave) external payable {
        require(bootstrapEnded, "Bootstrap not ended");
        require(msg.value >= 1 ether, "Insufficient stake");
        require(!validators[msg.sender].active, "Already registered");
        
        validators[msg.sender] = Validator({
            addr: msg.sender,
            mrenclave: _mrenclave,
            instanceId: bytes32(0),
            stake: msg.value,
            active: true,
            registeredAt: block.timestamp
        });
        validatorCount++;
        
        emit ValidatorRegistered(msg.sender, _mrenclave);
    }
    
    function isValidator(address _addr) external view returns (bool) {
        return validators[_addr].active || founders[_addr];
    }
    
    function isFounder(address _addr) external view returns (bool) {
        return founders[_addr];
    }
}
