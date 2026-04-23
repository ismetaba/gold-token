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
/// @notice Multi-sig + reserve-gated token issuance for GOLD (Ethereum mainnet).
/// @dev CRITICAL: totalSupply + amount must not exceed the latest attestedGrams.
///      Attestation freshness (maxReserveAge) is enforced at execution time.
///
///      Fee model: MINT_FEE_BPS = 25 (0.25%).
///      On executeMint, `fee = amount * MINT_FEE_BPS / 10_000` is minted to
///      feeRecipient (Treasury); the remainder goes to the intended recipient.
///      The reserve invariant is checked against the gross amount.
contract MintController is
    Initializable,
    AccessControlUpgradeable,
    ReentrancyGuardUpgradeable,
    UUPSUpgradeable,
    IMintController
{
    /// @notice Mint fee in basis points (25 bps = 0.25%).
    uint256 public constant MINT_FEE_BPS = 25;

    /// @custom:storage-location erc7201:gold.mint.storage
    struct MintStorage {
        IGoldToken token;
        IComplianceRegistry compliance;
        IReserveOracle oracle;
        address feeRecipient;               // receives mint fee tokens (typically Treasury)
        uint8 approvalThreshold;            // default 3
        uint8 totalApprovers;               // default 5
        uint256 maxReserveAge;              // seconds (35 days)
        mapping(bytes32 => Proposal) proposals;
        mapping(bytes32 => bool) allocationUsed;
        mapping(bytes32 => mapping(address => bool)) hasApproved;
        // Rate limiting
        uint256 rateLimitWindow;            // window length in seconds (0 = disabled)
        uint256 rateLimitMax;               // max gram-wei per window (0 = disabled)
        uint256 rateLimitWindowStart;       // start timestamp of the current window
        uint256 rateLimitMinted;            // amount minted in the current window
    }

    // keccak256(abi.encode(uint256(keccak256("gold.mint.storage")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant STORAGE_LOCATION =
        0xc4a8579f1e899c49fc9c3473bba3ff1123780e65bca83ece63cf77572c1fb100;

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
        $.feeRecipient = treasury;  // Treasury receives mint fees by default
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
    // Flow: propose → approve (k-of-n) → execute
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

        // proposalId == allocationId: single-use and deterministic
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

        // 1. Compliance: is the recipient allowed to receive a mint?
        if (!$.compliance.canMint(p.req.to, p.req.amount, p.req.jurisdiction)) {
            revert Errors.NotAuthorized();
        }

        // 2. Reserve freshness: latest PoR must be within maxReserveAge
        uint256 age = $.oracle.timeSinceLatest();
        if (age > $.maxReserveAge) {
            revert Errors.StaleReserveAttestation(block.timestamp - age, $.maxReserveAge);
        }

        // 3. Reserve invariant: totalSupply + grossAmount <= attestedGrams
        uint256 grossAmount = p.req.amount;
        uint256 supplyAfter = $.token.totalSupply() + grossAmount;
        uint256 attested = $.oracle.latestAttestedGrams();
        if (supplyAfter > attested) {
            revert Errors.ReserveInvariantViolated(supplyAfter, attested);
        }

        // 4. Rate limit: max minted per window (gross amount counts against limit)
        if ($.rateLimitMax > 0) {
            if (block.timestamp >= $.rateLimitWindowStart + $.rateLimitWindow) {
                $.rateLimitWindowStart = block.timestamp;
                $.rateLimitMinted = 0;
            }
            uint256 mintedAfter = $.rateLimitMinted + grossAmount;
            if (mintedAfter > $.rateLimitMax) {
                revert Errors.RateLimitExceeded(mintedAfter, $.rateLimitMax);
            }
            $.rateLimitMinted = mintedAfter;
        }

        // 5. Effects (CEI: update state before external interactions)
        p.status = ProposalStatus.EXECUTED;
        $.allocationUsed[p.req.allocationId] = true;

        // 6. Interaction: split mint between recipient and fee recipient
        uint256 fee = (grossAmount * MINT_FEE_BPS) / 10_000;
        uint256 toRecipient = grossAmount - fee;

        $.token.mint(p.req.to, toRecipient, p.req.jurisdiction);
        if (fee > 0) {
            $.token.mint($.feeRecipient, fee, p.req.jurisdiction);
            emit MintFeeCollected(proposalId, fee, $.feeRecipient);
        }

        emit MintExecuted(proposalId, grossAmount, supplyAfter);
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
    // Configuration
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

    /// @notice Update the fee recipient. Only TREASURY_ROLE.
    function setFeeRecipient(address newFeeRecipient) external onlyRole(Roles.TREASURY_ROLE) {
        if (newFeeRecipient == address(0)) revert Errors.ZeroAddress();
        MintStorage storage $ = _s();
        address old = $.feeRecipient;
        $.feeRecipient = newFeeRecipient;
        emit FeeRecipientUpdated(old, newFeeRecipient);
    }

    /// @notice Configure rate limiting. window=0 or max=0 disables the limit.
    function setRateLimit(uint256 window, uint256 max) external onlyRole(Roles.TREASURY_ROLE) {
        MintStorage storage $ = _s();
        $.rateLimitWindow = window;
        $.rateLimitMax = max;
        // Reset the window when a new limit is applied
        $.rateLimitWindowStart = block.timestamp;
        $.rateLimitMinted = 0;
        emit RateLimitUpdated(window, max);
    }

    // ──────────────────────────────────────────────────────────────────────
    // View
    // ──────────────────────────────────────────────────────────────────────

    function approvalThreshold() external view returns (uint8) {
        return _s().approvalThreshold;
    }

    function maxReserveAge() external view returns (uint256) {
        return _s().maxReserveAge;
    }

    function feeRecipient() external view returns (address) {
        return _s().feeRecipient;
    }

    function getProposal(bytes32 proposalId) external view returns (Proposal memory) {
        return _s().proposals[proposalId];
    }

    function isAllocationUsed(bytes32 allocationId) external view returns (bool) {
        return _s().allocationUsed[allocationId];
    }

    function rateLimit() external view returns (uint256 window, uint256 max) {
        MintStorage storage $ = _s();
        return ($.rateLimitWindow, $.rateLimitMax);
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
