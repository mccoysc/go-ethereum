// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity ^0.8.0;

/**
 * @title GovernanceContract
 * @dev 治理合约 - 管理 MRENCLAVE 白名单、验证者和投票
 */
contract GovernanceContract {
    // 提案类型
    enum ProposalType {
        AddMREnclave,
        RemoveMREnclave,
        UpgradeMREnclave,
        AddValidator,
        RemoveValidator,
        UpdateParameter
    }

    // 提案状态
    enum ProposalStatus {
        Pending,
        Active,
        Passed,
        Rejected,
        Executed,
        Vetoed
    }

    // 提案结构
    struct Proposal {
        uint256 id;
        ProposalType proposalType;
        address proposer;
        bytes32 target; // MRENCLAVE 或其他目标
        uint256 startTime;
        uint256 endTime;
        uint256 yesVotes;
        uint256 noVotes;
        ProposalStatus status;
        bool executed;
    }

    // 验证者信息
    struct Validator {
        address addr;
        bytes32 mrenclave;
        uint256 stake;
        bool active;
        uint256 registeredAt;
    }

    // 状态变量
    mapping(bytes32 => bool) public mrenclaveWhitelist;
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(address => Validator) public validators;
    
    uint256 public proposalCount;
    uint256 public validatorCount;
    uint256 public constant VOTING_PERIOD = 7 days;
    uint256 public constant MIN_STAKE = 1 ether;
    
    address[] public validatorList;
    bytes32[] public whitelistedMREnclaves;

    // 事件
    event ProposalCreated(uint256 indexed proposalId, ProposalType proposalType, address proposer);
    event Voted(uint256 indexed proposalId, address indexed voter, bool support);
    event ProposalExecuted(uint256 indexed proposalId);
    event MREnclaveAdded(bytes32 indexed mrenclave);
    event MREnclaveRemoved(bytes32 indexed mrenclave);
    event ValidatorRegistered(address indexed validator, bytes32 mrenclave);
    event ValidatorRemoved(address indexed validator);

    constructor() {
        // 初始化：添加创世 MRENCLAVE
        bytes32 genesisMREnclave = bytes32(uint256(0x1234567890abcdef));
        mrenclaveWhitelist[genesisMREnclave] = true;
        whitelistedMREnclaves.push(genesisMREnclave);
    }

    // 创建提案
    function createProposal(
        ProposalType _type,
        bytes32 _target
    ) external returns (uint256) {
        require(validators[msg.sender].active, "Only validators can create proposals");
        
        proposalCount++;
        proposals[proposalCount] = Proposal({
            id: proposalCount,
            proposalType: _type,
            proposer: msg.sender,
            target: _target,
            startTime: block.timestamp,
            endTime: block.timestamp + VOTING_PERIOD,
            yesVotes: 0,
            noVotes: 0,
            status: ProposalStatus.Active,
            executed: false
        });

        emit ProposalCreated(proposalCount, _type, msg.sender);
        return proposalCount;
    }

    // 投票
    function vote(uint256 _proposalId, bool _support) external {
        require(validators[msg.sender].active, "Only validators can vote");
        require(!hasVoted[_proposalId][msg.sender], "Already voted");
        require(proposals[_proposalId].status == ProposalStatus.Active, "Proposal not active");
        require(block.timestamp <= proposals[_proposalId].endTime, "Voting period ended");

        hasVoted[_proposalId][msg.sender] = true;
        
        if (_support) {
            proposals[_proposalId].yesVotes += validators[msg.sender].stake;
        } else {
            proposals[_proposalId].noVotes += validators[msg.sender].stake;
        }

        emit Voted(_proposalId, msg.sender, _support);
    }

    // 执行提案
    function executeProposal(uint256 _proposalId) external {
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.status == ProposalStatus.Active, "Proposal not active");
        require(block.timestamp > proposal.endTime, "Voting period not ended");
        require(!proposal.executed, "Already executed");

        // 检查是否通过（简单多数）
        if (proposal.yesVotes > proposal.noVotes) {
            proposal.status = ProposalStatus.Passed;
            
            // 执行提案
            if (proposal.proposalType == ProposalType.AddMREnclave) {
                _addMREnclave(proposal.target);
            } else if (proposal.proposalType == ProposalType.RemoveMREnclave) {
                _removeMREnclave(proposal.target);
            }
            
            proposal.executed = true;
            emit ProposalExecuted(_proposalId);
        } else {
            proposal.status = ProposalStatus.Rejected;
        }
    }

    // 注册验证者
    function registerValidator(bytes32 _mrenclave) external payable {
        require(msg.value >= MIN_STAKE, "Insufficient stake");
        require(!validators[msg.sender].active, "Already registered");
        require(mrenclaveWhitelist[_mrenclave], "MRENCLAVE not whitelisted");

        validators[msg.sender] = Validator({
            addr: msg.sender,
            mrenclave: _mrenclave,
            stake: msg.value,
            active: true,
            registeredAt: block.timestamp
        });

        validatorList.push(msg.sender);
        validatorCount++;

        emit ValidatorRegistered(msg.sender, _mrenclave);
    }

    // 内部函数：添加 MRENCLAVE
    function _addMREnclave(bytes32 _mrenclave) internal {
        require(!mrenclaveWhitelist[_mrenclave], "Already whitelisted");
        mrenclaveWhitelist[_mrenclave] = true;
        whitelistedMREnclaves.push(_mrenclave);
        emit MREnclaveAdded(_mrenclave);
    }

    // 内部函数：移除 MRENCLAVE
    function _removeMREnclave(bytes32 _mrenclave) internal {
        require(mrenclaveWhitelist[_mrenclave], "Not whitelisted");
        mrenclaveWhitelist[_mrenclave] = false;
        emit MREnclaveRemoved(_mrenclave);
    }

    // 查询函数
    function isWhitelisted(bytes32 _mrenclave) external view returns (bool) {
        return mrenclaveWhitelist[_mrenclave];
    }

    function isValidator(address _addr) external view returns (bool) {
        return validators[_addr].active;
    }

    function getProposal(uint256 _proposalId) external view returns (Proposal memory) {
        return proposals[_proposalId];
    }

    function getValidatorCount() external view returns (uint256) {
        return validatorCount;
    }

    function getWhitelistedMREnclaves() external view returns (bytes32[] memory) {
        return whitelistedMREnclaves;
    }
}
