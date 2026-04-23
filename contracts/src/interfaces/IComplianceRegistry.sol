// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IComplianceRegistry
/// @notice Central KYC/AML gate for GOLD token transfers.
/// @dev GoldToken._update calls canTransfer() on every token movement. Registry writes
///      are performed by the off-chain KYC Service. Freezing is a manual Compliance
///      Officer action.
interface IComplianceRegistry {
    enum KycTier {
        NONE,           // unregistered
        BASIC,          // light — small limits
        ENHANCED,       // full — large limits
        INSTITUTIONAL   // institutional — no limits (operational)
    }

    struct WalletProfile {
        KycTier tier;
        bytes2 jurisdiction;        // ISO-3166 alpha-2: "TR","CH","AE","LI","DE","FR"...
        uint64 kycApprovedAt;
        uint64 kycExpiresAt;
        bool frozen;                // court order / manual compliance action
        bool sanctioned;            // OFAC/EU/UN match
    }

    /// @notice Does this transfer satisfy compliance rules?
    /// @dev Called by GoldToken in the _update hook.
    ///      Callers skip this when from=0 (mint) or to=0 (burn).
    function canTransfer(address from, address to, uint256 amount) external view returns (bool);

    /// @notice May this address receive a mint? (KYC + jurisdiction check)
    function canMint(address to, uint256 amount, bytes2 jurisdiction) external view returns (bool);

    /// @notice May this address burn tokens? (allowed unless frozen or sanctioned)
    function canBurn(address from, uint256 amount) external view returns (bool);

    /// @notice Is a Travel Rule message a prerequisite for this transfer?
    /// @dev Required when the amount exceeds the configured threshold.
    function travelRuleRequired(address from, address to, uint256 amount)
        external
        view
        returns (bool);

    /// @notice Record an IVMS101 message hash from the counterparty VASP.
    function recordTravelRuleApproval(
        address from,
        address to,
        uint256 amount,
        bytes32 ivms101Hash
    ) external;

    // Profile management — KYC_WRITER_ROLE only
    function setProfile(address wallet, WalletProfile calldata profile) external;
    function getProfile(address wallet) external view returns (WalletProfile memory);

    // Freeze / unfreeze — COMPLIANCE_OFFICER_ROLE only
    function freeze(address wallet, bytes32 reasonHash) external;
    function unfreeze(address wallet) external;

    // Sanctions update
    function setSanctioned(address wallet, bool value) external;

    // View helpers
    function isKycValid(address wallet) external view returns (bool);
    function isFrozen(address wallet) external view returns (bool);
    function isSanctioned(address wallet) external view returns (bool);

    // Jurisdiction restrictions (e.g. block US residents)
    function setJurisdictionBlocked(bytes2 jurisdiction, bool blocked) external;
    function isJurisdictionBlocked(bytes2 jurisdiction) external view returns (bool);

    // Travel Rule threshold (amount in wei; 1 gram = 1e18)
    function setTravelRuleThreshold(uint256 thresholdWei) external;
    function travelRuleThreshold() external view returns (uint256);

    event ProfileUpdated(address indexed wallet, KycTier tier, bytes2 jurisdiction, uint64 expiresAt);
    event Frozen(address indexed wallet, bytes32 reasonHash);
    event Unfrozen(address indexed wallet);
    event SanctionsUpdated(address indexed wallet, bool sanctioned);
    event JurisdictionBlockUpdated(bytes2 jurisdiction, bool blocked);
    event TravelRuleRecorded(
        address indexed from,
        address indexed to,
        uint256 amount,
        bytes32 ivms101Hash
    );
    event TravelRuleThresholdUpdated(uint256 newThreshold);
}
