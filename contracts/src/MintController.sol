// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { AccessControlUpgradeable } from
    "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from
    "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";

import { IMintController } from "./interfaces/IMintController.sol";
import { IGoldToken } from "./interfaces/IGoldToken.sol";
import { IComplianceRegistry } from "./interfaces/IComplianceRegistry.sol";
import { IReserveOracle } from "./interfaces/IReserveOracle.sol";
import { Errors } from "./libraries/Errors.sol";
import { Roles } from "./libraries/Roles.sol";

/// @title MintController
/// @notice Çoklu imza + rezerv-kapılı token basımı.
/// @dev KRİTİK: totalSupply artı amount, son attestedGrams'ı aşmamalı.
///      Son atestasyon tazeliği (maxReserveAge) zorunlu.
contract MintController is
    Initializable,
    AccessControlUpgradeable,
    ReentrancyGuardUpgradeable,
    UUPSUpgradeable,
    IMintController
{
    /// @custom:storage-location erc7201:gold.mint.storage
    struct MintStorage {
        IGoldToken token;
        IComplianceRegistry compliance;
        IReserveOracle oracle;
        uint8 approvalThreshold;            // tipik 3
        uint8 totalApprovers;               // tipik 5
        uint256 maxReserveAge;              // saniye (35 gün)
        mapping(bytes32 => Proposal) proposals;
        mapping(bytes32 => bool) allocationUsed;
        mapping(bytes32 => mapping(address => bool)) hasApproved;
    }

    // keccak256(abi.encode(uint256(keccak256("gold.mint.storage")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant STORAGE_LOCATION =
        0xc3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c300;

    function _s() private pure returns (MintStorage storage $) {
        assembly {
            $.slot := STORAGE_LOCATION
        }
    }

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(
        address treasury,
        address token_,
        address compliance_,
        address oracle_,
        address[] calldata approvers,
        address proposer,
        address executor,
        uint8 approvalThreshold_,
        uint256 maxReserveAge_
    ) external initializer {
        if (
            treasury == address(0) || token_ == address(0) || compliance_ == address(0)
                || oracle_ == address(0) || proposer == address(0) || executor == address(0)
        ) revert Errors.ZeroAddress();
        if (approvalThreshold_ == 0 || approvalThreshold_ > approvers.length) {
            revert Errors.InsufficientApprovals(0, approvalThreshold_);
        }

        __AccessControl_init();
        __ReentrancyGuard_init();
        __UUPSUpgradeable_init();

        MintStorage storage $ = _s();
        $.token = IGoldToken(token_);
        $.compliance = IComplianceRegistry(compliance_);
        $.oracle = IReserveOracle(oracle_);
        $.approvalThreshold = approvalThreshold_;
        $.totalApprovers = uint8(approvers.length);
        $.maxReserveAge = maxReserveAge_;

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);
        _grantRole(Roles.UPGRADER_ROLE, treasury);
        _grantRole(Roles.MINT_PROPOSER_ROLE, proposer);
        _grantRole(Roles.MINT_EXECUTOR_ROLE, executor);

        for (uint256 i = 0; i < approvers.length; i++) {
            if (approvers[i] == address(0)) revert Errors.ZeroAddress();
            _grantRole(Roles.MINT_APPROVER_ROLE, approvers[i]);
        }
    }

    // ──────────────────────────────────────────────────────────────────────
    // Akış: propose → approve (k-of-n) → execute
    // ──────────────────────────────────────────────────────────────────────

    function proposeMint(MintRequest calldata req)
        external
        onlyRole(Roles.MINT_PROPOSER_ROLE)
        returns (bytes32 proposalId)
    {
        if (req.to == address(0)) revert Errors.ZeroAddress();
        if (req.amount == 0) revert Errors.ZeroAmount();
        if (req.barSerials.length == 0) revert Errors.EmptyBarList();

        MintStorage storage $ = _s();
        if ($.allocationUsed[req.allocationId]) {
            revert Errors.AllocationAlreadyUsed(req.allocationId);
        }

        // proposalId = allocationId — tek-kullanımlık ve tahminli
        proposalId = req.allocationId;

        Proposal storage p = $.proposals[proposalId];
        p.req = req;
        p.status = ProposalStatus.PROPOSED;
        p.proposer = msg.sender;

        emit MintProposed(
            proposalId, msg.sender, req.to, req.amount, req.jurisdiction, req.allocationId
        );
    }

    function approveMint(bytes32 proposalId) external onlyRole(Roles.MINT_APPROVER_ROLE) {
        MintStorage storage $ = _s();
        Proposal storage p = $.proposals[proposalId];
        if (p.status != ProposalStatus.PROPOSED) revert Errors.ProposalNotFound(proposalId);
        if ($.hasApproved[proposalId][msg.sender]) {
            revert Errors.ProposalAlreadyApprovedBy(msg.sender);
        }

        $.hasApproved[proposalId][msg.sender] = true;
        p.approvers.push(msg.sender);

        emit MintApproved(proposalId, msg.sender, uint8(p.approvers.length));
    }

    function executeMint(bytes32 proposalId)
        external
        nonReentrant
        onlyRole(Roles.MINT_EXECUTOR_ROLE)
    {
        MintStorage storage $ = _s();
        Proposal storage p = $.proposals[proposalId];

        if (p.status != ProposalStatus.PROPOSED) revert Errors.ProposalNotFound(proposalId);
        if (p.approvers.length < $.approvalThreshold) {
            revert Errors.InsufficientApprovals(p.approvers.length, $.approvalThreshold);
        }

        // 1. Compliance: alıcı mint edebilir durumda mı?
        if (!$.compliance.canMint(p.req.to, p.req.amount, p.req.jurisdiction)) {
            revert Errors.NotAuthorized();
        }

        // 2. Rezerv tazeliği: son PoR ≤ maxReserveAge
        uint256 age = $.oracle.timeSinceLatest();
        if (age > $.maxReserveAge) {
            revert Errors.StaleReserveAttestation(block.timestamp - age, $.maxReserveAge);
        }

        // 3. Rezerv invaryantı: totalSupply + amount ≤ attestedGrams
        uint256 supplyAfter = $.token.totalSupply() + p.req.amount;
        uint256 attested = $.oracle.latestAttestedGrams();
        if (supplyAfter > attested) {
            revert Errors.ReserveInvariantViolated(supplyAfter, attested);
        }

        // 4. Durumu efect'lerden önce güncelle (CEI)
        p.status = ProposalStatus.EXECUTED;
        $.allocationUsed[p.req.allocationId] = true;

        // 5. Etkileşim: mint
        $.token.mint(p.req.to, p.req.amount, p.req.jurisdiction);

        emit MintExecuted(proposalId, p.req.amount, supplyAfter);
    }

    function cancelMint(bytes32 proposalId, bytes32 reasonHash) external {
        MintStorage storage $ = _s();
        Proposal storage p = $.proposals[proposalId];
        if (p.status != ProposalStatus.PROPOSED) revert Errors.ProposalNotFound(proposalId);

        bool auth = hasRole(Roles.COMPLIANCE_OFFICER_ROLE, msg.sender)
            || hasRole(Roles.TREASURY_ROLE, msg.sender) || msg.sender == p.proposer;
        if (!auth) revert Errors.NotAuthorized();

        p.status = ProposalStatus.CANCELLED;
        emit MintCancelled(proposalId, reasonHash);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Konfigürasyon
    // ──────────────────────────────────────────────────────────────────────

    function setApprovalThreshold(uint8 threshold) external onlyRole(Roles.TREASURY_ROLE) {
        MintStorage storage $ = _s();
        if (threshold == 0 || threshold > $.totalApprovers) {
            revert Errors.InsufficientApprovals(0, threshold);
        }
        $.approvalThreshold = threshold;
        emit ApprovalThresholdUpdated(threshold);
    }

    function setMaxReserveAge(uint256 ageSeconds) external onlyRole(Roles.TREASURY_ROLE) {
        _s().maxReserveAge = ageSeconds;
        emit MaxReserveAgeUpdated(ageSeconds);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Görünüm
    // ──────────────────────────────────────────────────────────────────────

    function approvalThreshold() external view returns (uint8) {
        return _s().approvalThreshold;
    }

    function maxReserveAge() external view returns (uint256) {
        return _s().maxReserveAge;
    }

    function getProposal(bytes32 proposalId) external view returns (Proposal memory) {
        return _s().proposals[proposalId];
    }

    function isAllocationUsed(bytes32 allocationId) external view returns (bool) {
        return _s().allocationUsed[allocationId];
    }

    // ──────────────────────────────────────────────────────────────────────
    // UUPS
    // ──────────────────────────────────────────────────────────────────────

    function _authorizeUpgrade(address newImpl)
        internal
        override
        onlyRole(Roles.UPGRADER_ROLE)
    {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
    }
}
