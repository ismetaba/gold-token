// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IBurnController
/// @notice Two redemption paths:
///         1. User redemption — user burns tokens, off-chain vault releases gold / cash
///         2. Operator burn  — operational correction (BURN_OPERATOR_ROLE only)
///
/// Fee model:
///      A BURN_FEE_BPS (25 bps = 0.25%) fee is deducted from every redemption.
///      The full req.amount is burned on-chain; BurnFeeCollected reports the implied fee
///      so the off-chain settlement system can deliver req.amount - fee grams / cash.
interface IBurnController {
    enum RedemptionType {
        CASH_BACK,      // fiat settlement (TRY / USD / EUR / AED)
        PHYSICAL        // physical gold delivery (min 100 grams)
    }

    struct RedemptionRequest {
        address from;
        uint256 amount;             // gross gram-wei burned on-chain
        RedemptionType redemptionType;
        bytes32 offChainOrderId;    // backend order ID (UUID)
        string deliveryRef;         // IBAN / address hash etc. (off-chain reference)
    }

    /// @notice User redemption burn — approve + burn.
    /// @dev User must first call GoldToken.approve(burnController, amount).
    ///      The backend (BURN_OPERATOR_ROLE) then calls this function.
    ///      A BurnFeeCollected event is emitted; off-chain settlement delivers
    ///      amount - fee grams / cash to the user.
    function requestRedemption(RedemptionRequest calldata req) external returns (bytes32 reqId);

    /// @notice Minimum token amount for a PHYSICAL redemption.
    function minPhysicalGrams() external view returns (uint256);

    /// @notice Operator burn — operational correction (e.g. reversing an erroneous mint).
    ///         Requires BURN_OPERATOR_ROLE + a valid COMPLIANCE_OFFICER_ROLE signature.
    /// @param deadline Signature expiry (Unix timestamp). Reverts if block.timestamp > deadline.
    function operatorBurn(
        address from,
        uint256 amount,
        bytes32 reasonHash,
        uint256 deadline,
        bytes calldata complianceOfficerSig
    ) external;

    // View
    function getRedemption(bytes32 reqId)
        external
        view
        returns (RedemptionRequest memory, bool executed, uint256 executedAt);

    event RedemptionRequested(
        bytes32 indexed reqId,
        address indexed from,
        uint256 amount,
        RedemptionType redemptionType,
        bytes32 offChainOrderId
    );
    event RedemptionExecuted(bytes32 indexed reqId, uint256 newTotalSupply);
    /// @notice Emitted on every redemption to report the 0.25% fee deducted from delivery.
    /// @param reqId   Redemption request ID.
    /// @param from    User who burned tokens.
    /// @param fee     Fee amount in gram-wei (off-chain settlement delivers amount - fee).
    event BurnFeeCollected(bytes32 indexed reqId, address indexed from, uint256 fee);
    event OperatorBurn(address indexed from, uint256 amount, bytes32 reasonHash);
    event MinPhysicalGramsUpdated(uint256 newMin);
}
