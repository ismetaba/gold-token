// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

library Roles {
    /// @dev Treasury Safe (3-of-5). Contract ownership, parameter changes, emergency.
    bytes32 internal constant TREASURY_ROLE = keccak256("gold.role.treasury");

    /// @dev May upgrade contracts. Requires Timelock + Treasury approval.
    bytes32 internal constant UPGRADER_ROLE = keccak256("gold.role.upgrader");

    /// @dev May pause in an emergency. Only pause; unpause is restricted to Treasury.
    bytes32 internal constant PAUSER_ROLE = keccak256("gold.role.pauser");

    /// @dev KYC Service backend. May write wallet profiles.
    bytes32 internal constant KYC_WRITER_ROLE = keccak256("gold.role.kyc_writer");

    /// @dev Compliance Officer. May freeze/unfreeze wallets, manual review actions.
    bytes32 internal constant COMPLIANCE_OFFICER_ROLE = keccak256("gold.role.compliance_officer");

    /// @dev May propose new mint requests (Mint/Burn Service backend).
    bytes32 internal constant MINT_PROPOSER_ROLE = keccak256("gold.role.mint_proposer");

    /// @dev May approve mint proposals (5 seats; 3-of-5 threshold required).
    bytes32 internal constant MINT_APPROVER_ROLE = keccak256("gold.role.mint_approver");

    /// @dev May execute an approved mint after the threshold is met.
    bytes32 internal constant MINT_EXECUTOR_ROLE = keccak256("gold.role.mint_executor");

    /// @dev Operator burn — post-redemption vault deduction.
    bytes32 internal constant BURN_OPERATOR_ROLE = keccak256("gold.role.burn_operator");

    /// @dev May publish reserve attestations (Big Four auditor wallet).
    bytes32 internal constant AUDITOR_ROLE = keccak256("gold.role.auditor");
}
