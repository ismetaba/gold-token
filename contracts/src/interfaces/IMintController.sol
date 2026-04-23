// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IMintController
/// @notice Multi-sig + reserve-gated token issuance.
/// @dev Critical invariants:
///      1. Each mint requires a fresh PoR attestation: age <= maxReserveAge (35 days)
///      2. After mint: totalSupply <= ReserveOracle.latestAttestedGrams()
///      3. Requires k-of-n approvals (default 3-of-5)
///      4. Each allocationId is single-use — double-mint is impossible
///
/// Fee model:
///      A MINT_FEE_BPS (25 bps = 0.25%) fee is deducted from every mint.
///      The fee portion is minted to feeRecipient (Treasury); the remainder goes to the
///      intended recipient.  The reserve invariant is checked against the gross amount.
interface IMintController {
    enum ProposalStatus {
        NONE,
        PROPOSED,
        EXECUTED,
        CANCELLED
    }

    struct MintRequest {
        address to;                 // destination wallet (must be KYC-verified)
        uint256 amount;             // gross gram-wei (1e18 = 1 g); fee is deducted from this
        bytes32 allocationId;       // off-chain UUID (double-mint protection)
        bytes32[] barSerials;       // hashes of vault bars backing this mint
        bytes2 jurisdiction;        // ISO-3166: TR / CH / AE / LI …
        uint64 proposedAt;
    }

    struct Proposal {
        MintRequest req;
        ProposalStatus status;
        address proposer;
        address[] approvers;        // deduplicated list of approving addresses
    }

    /// @notice Open a new mint proposal. MINT_PROPOSER_ROLE only.
    function proposeMint(MintRequest calldata req) external returns (bytes32 proposalId);

    /// @notice Approve a proposal. MINT_APPROVER_ROLE only. An approver cannot vote twice.
    function approveMint(bytes32 proposalId) external;

    /// @notice Execute a mint once the approval threshold is met. MINT_EXECUTOR_ROLE only.
    /// @dev Checks the reserve invariant at execution time.
    function executeMint(bytes32 proposalId) external;

    /// @notice Cancel a proposal (COMPLIANCE_OFFICER_ROLE or proposer).
    function cancelMint(bytes32 proposalId, bytes32 reasonHash) external;

    // Configuration
    function setApprovalThreshold(uint8 threshold) external;
    function setMaxReserveAge(uint256 ageSeconds) external;
    function setRateLimit(uint256 window, uint256 max) external;
    function setFeeRecipient(address newFeeRecipient) external;

    // View
    function approvalThreshold() external view returns (uint8);
    function totalApprovers() external view returns (uint8);
    function maxReserveAge() external view returns (uint256);
    function feeRecipient() external view returns (address);
    function rateLimit() external view returns (uint256 window, uint256 max);
    function getProposal(bytes32 proposalId) external view returns (Proposal memory);
    function isAllocationUsed(bytes32 allocationId) external view returns (bool);

    event MintProposed(
        bytes32 indexed proposalId,
        address indexed proposer,
        address indexed to,
        uint256 amount,
        bytes2 jurisdiction,
        bytes32 allocationId
    );
    event MintApproved(bytes32 indexed proposalId, address indexed approver, uint8 approvalCount);
    event MintExecuted(bytes32 indexed proposalId, uint256 amount, uint256 newTotalSupply);
    event MintCancelled(bytes32 indexed proposalId, bytes32 reasonHash);
    /// @notice Emitted when the mint fee is collected.
    /// @param proposalId  The executed proposal.
    /// @param fee         Fee amount in gram-wei minted to feeRecipient.
    /// @param feeRecipient Address that received the fee tokens.
    event MintFeeCollected(bytes32 indexed proposalId, uint256 fee, address indexed feeRecipient);
    event ApprovalThresholdUpdated(uint8 newThreshold);
    event MaxReserveAgeUpdated(uint256 newAgeSeconds);
    event RateLimitUpdated(uint256 window, uint256 max);
    event FeeRecipientUpdated(address indexed oldRecipient, address indexed newRecipient);
}
