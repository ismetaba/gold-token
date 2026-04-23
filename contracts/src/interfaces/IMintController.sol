// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IMintController
/// @notice Çoklu imza + rezerv-kapılı token basımı.
/// @dev Kritik invaryantlar:
///      1. Her mint için son PoR atestasyonu ≤ maxReserveAge (35 gün)
///      2. Mint sonrası totalSupply ≤ ReserveOracle.latestAttestedGrams()
///      3. 3/5 onay gerekir (5 ayrı adres)
///      4. Her allocationId tek kullanımlık — çifte mint engellenir
interface IMintController {
    enum ProposalStatus {
        NONE,
        PROPOSED,
        EXECUTED,
        CANCELLED
    }

    struct MintRequest {
        address to;                 // hedef cüzdan (KYC'li olmalı)
        uint256 amount;             // gram wei (1e18 = 1g)
        bytes32 allocationId;       // off-chain UUID (çifte mint koruması)
        bytes32[] barSerials;       // bu mint'i destekleyen kasa çubukları (seri hash'leri)
        bytes2 jurisdiction;        // TR/CH/AE/LI
        uint64 proposedAt;
    }

    struct Proposal {
        MintRequest req;
        ProposalStatus status;
        address proposer;
        address[] approvers;        // onaylayan adresler listesi (dedupe edilmiş)
    }

    /// @notice Yeni mint teklifi açar. Sadece MINT_PROPOSER_ROLE.
    function proposeMint(MintRequest calldata req) external returns (bytes32 proposalId);

    /// @notice Teklifi onaylar. Sadece MINT_APPROVER_ROLE. Tek onaylayıcı iki kez onaylayamaz.
    function approveMint(bytes32 proposalId) external;

    /// @notice Eşik sağlandıktan sonra basımı çalıştırır. Sadece MINT_EXECUTOR_ROLE.
    /// @dev Rezerv invaryantını çalıştırma anında kontrol eder.
    function executeMint(bytes32 proposalId) external;

    /// @notice Teklifi iptal eder (COMPLIANCE_OFFICER_ROLE veya proposer).
    function cancelMint(bytes32 proposalId, bytes32 reasonHash) external;

    // Konfigürasyon
    function setApprovalThreshold(uint8 threshold) external;
    function setMaxReserveAge(uint256 ageSeconds) external;
    function setRateLimit(uint256 window, uint256 max) external;

    // Görünüm
    function approvalThreshold() external view returns (uint8);
    function maxReserveAge() external view returns (uint256);
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
    event ApprovalThresholdUpdated(uint8 newThreshold);
    event MaxReserveAgeUpdated(uint256 newAgeSeconds);
    event RateLimitUpdated(uint256 window, uint256 max);
}
