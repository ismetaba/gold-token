// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IComplianceRegistry
/// @notice GOLD token transferleri için merkezi uyum (KYC/AML) kapısı.
/// @dev GoldToken._update içinden canTransfer() çağrılır. Registry yazımları off-chain
///      KYC Service tarafından yapılır. Dondurma manuel Compliance Officer kararıyla.
interface IComplianceRegistry {
    enum KycTier {
        NONE,           // kayıtsız
        BASIC,          // hafif — küçük limit
        ENHANCED,       // tam — büyük limit
        INSTITUTIONAL   // kurumsal — limitsiz (operatif)
    }

    struct WalletProfile {
        KycTier tier;
        bytes2 jurisdiction;        // ISO-3166 alpha-2: "TR","CH","AE","LI","DE","FR"...
        uint64 kycApprovedAt;
        uint64 kycExpiresAt;
        bool frozen;                // mahkeme emri / manuel
        bool sanctioned;            // OFAC/EU/UN eşleşmesi
    }

    /// @notice Bu transfer uyum kurallarını karşılıyor mu?
    /// @dev GoldToken tarafından _update hook'unda çağrılır.
    ///      from=0 (mint) veya to=0 (burn) durumlarını çağıran es geçer.
    function canTransfer(address from, address to, uint256 amount) external view returns (bool);

    /// @notice Mint yapılabilir mi? (KYC + jurisdiction check)
    function canMint(address to, uint256 amount, bytes2 jurisdiction) external view returns (bool);

    /// @notice Burn yapılabilir mi? (frozen değilse kullanıcı kendini yakabilir)
    function canBurn(address from, uint256 amount) external view returns (bool);

    /// @notice Transfer için Travel Rule mesajı ön koşul mu?
    /// @dev Tutar belirli eşiği aşıyorsa counterparty VASP mesajı bekler.
    function travelRuleRequired(address from, address to, uint256 amount)
        external
        view
        returns (bool);

    /// @notice Counterparty VASP'den IVMS101 mesajını hash olarak kaydet.
    function recordTravelRuleApproval(
        address from,
        address to,
        uint256 amount,
        bytes32 ivms101Hash
    ) external;

    // Profil yönetimi — sadece KYC_WRITER_ROLE
    function setProfile(address wallet, WalletProfile calldata profile) external;
    function getProfile(address wallet) external view returns (WalletProfile memory);

    // Dondurma — sadece COMPLIANCE_OFFICER_ROLE
    function freeze(address wallet, bytes32 reasonHash) external;
    function unfreeze(address wallet) external;

    // Toplu sanction güncellemesi
    function setSanctioned(address wallet, bool value) external;

    // Görünüm yardımcıları
    function isKycValid(address wallet) external view returns (bool);
    function isFrozen(address wallet) external view returns (bool);
    function isSanctioned(address wallet) external view returns (bool);

    // Jurisdiction kısıtlamaları (örn. ABD yerleşiklere yasak)
    function setJurisdictionBlocked(bytes2 jurisdiction, bool blocked) external;
    function isJurisdictionBlocked(bytes2 jurisdiction) external view returns (bool);

    // Travel Rule parametresi (sentler cinsinden değil; amount = wei cinsinden gram * 1e18)
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
