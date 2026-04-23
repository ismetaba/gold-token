// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

library Errors {
    error NotAuthorized();
    error ZeroAddress();
    error ZeroAmount();

    // Compliance
    error KycRequired(address wallet);
    error KycExpired(address wallet);
    error WalletFrozen(address wallet);
    error JurisdictionBlocked(bytes2 jurisdiction);
    error SanctionsHit(address wallet);
    error TravelRuleRequired(address from, address to, uint256 amount);

    // Mint/Burn
    error StaleReserveAttestation(uint256 lastAttestationAt, uint256 maxAge);
    error ReserveInvariantViolated(uint256 supplyAfter, uint256 attestedGrams);
    error ProposalNotFound(bytes32 proposalId);
    error ProposalAlreadyExecuted(bytes32 proposalId);
    error ProposalAlreadyApprovedBy(address approver);
    error InsufficientApprovals(uint256 current, uint256 required);
    error AllocationAlreadyUsed(bytes32 allocationId);
    error EmptyBarList();

    // Reserve Oracle
    error AttestationMonotonicityViolated(uint64 previousTimestamp, uint64 newTimestamp);
    error InvalidAuditorSignature();
    error UnknownAuditor(address signer);
    error InvalidMerkleProof();

    // Upgrade
    error UpgradeNotTimelocked();
    error UpgradeTimelockActive(uint256 until);
}
