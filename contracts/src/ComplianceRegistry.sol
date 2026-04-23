// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { AccessControlUpgradeable } from
    "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

import { IComplianceRegistry } from "./interfaces/IComplianceRegistry.sol";
import { Errors } from "./libraries/Errors.sol";
import { Roles } from "./libraries/Roles.sol";

/// @title ComplianceRegistry
/// @notice GOLD için merkezi uyum durumu. GoldToken _update içinden sorgular.
contract ComplianceRegistry is
    Initializable,
    AccessControlUpgradeable,
    UUPSUpgradeable,
    IComplianceRegistry
{
    /// @custom:storage-location erc7201:gold.compliance.storage
    struct RegStorage {
        mapping(address => WalletProfile) profiles;
        mapping(bytes2 => bool) jurisdictionBlocked;
        mapping(bytes32 => bool) travelRuleApproved; // hash(from,to,amount) => approved
        uint256 travelRuleThreshold;
    }

    // keccak256(abi.encode(uint256(keccak256("gold.compliance.storage")) - 1)) & ~bytes32(uint256(0xff))
    // TODO(deploy): precompute.
    bytes32 private constant STORAGE_LOCATION =
        0xb2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b200;

    function _s() private pure returns (RegStorage storage $) {
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
        address kycWriter,
        address complianceOfficer,
        uint256 travelRuleThreshold_
    ) external initializer {
        if (
            treasury == address(0) || kycWriter == address(0)
                || complianceOfficer == address(0)
        ) revert Errors.ZeroAddress();

        __AccessControl_init();
        __UUPSUpgradeable_init();

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);
        _grantRole(Roles.UPGRADER_ROLE, treasury);
        _grantRole(Roles.KYC_WRITER_ROLE, kycWriter);
        _grantRole(Roles.COMPLIANCE_OFFICER_ROLE, complianceOfficer);

        _s().travelRuleThreshold = travelRuleThreshold_;
        emit TravelRuleThresholdUpdated(travelRuleThreshold_);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Ana kapı — GoldToken._update tarafından çağrılır
    // ──────────────────────────────────────────────────────────────────────

    function canTransfer(address from, address to, uint256 amount)
        external
        view
        returns (bool)
    {
        RegStorage storage $ = _s();
        WalletProfile storage pf = $.profiles[from];
        WalletProfile storage pt = $.profiles[to];

        if (pf.frozen || pt.frozen) return false;
        if (pf.sanctioned || pt.sanctioned) return false;
        if ($.jurisdictionBlocked[pf.jurisdiction]) return false;
        if ($.jurisdictionBlocked[pt.jurisdiction]) return false;

        if (pf.tier == KycTier.NONE || pt.tier == KycTier.NONE) return false;
        if (pf.kycExpiresAt <= block.timestamp) return false;
        if (pt.kycExpiresAt <= block.timestamp) return false;

        // Travel Rule: eşiği aşan transferlerde counterparty onayı gerekir.
        if (amount >= $.travelRuleThreshold) {
            bytes32 key = _travelRuleKey(from, to, amount);
            if (!$.travelRuleApproved[key]) return false;
        }

        return true;
    }

    function canMint(address to, uint256, bytes2 jurisdiction)
        external
        view
        returns (bool)
    {
        RegStorage storage $ = _s();
        WalletProfile storage p = $.profiles[to];
        if (p.frozen || p.sanctioned) return false;
        if ($.jurisdictionBlocked[jurisdiction]) return false;
        if (p.tier == KycTier.NONE) return false;
        if (p.kycExpiresAt <= block.timestamp) return false;
        // Mint'te counterparty yok — Travel Rule uygulanmaz.
        return true;
    }

    function canBurn(address from, uint256) external view returns (bool) {
        RegStorage storage $ = _s();
        WalletProfile storage p = $.profiles[from];
        // Kullanıcı kendini yakıp itfa alabilir — frozen/sanctioned değilse.
        if (p.frozen || p.sanctioned) return false;
        return true;
    }

    function travelRuleRequired(address from, address to, uint256 amount)
        external
        view
        returns (bool)
    {
        RegStorage storage $ = _s();
        if (amount < $.travelRuleThreshold) return false;
        bytes32 key = _travelRuleKey(from, to, amount);
        return !$.travelRuleApproved[key];
    }

    function recordTravelRuleApproval(
        address from,
        address to,
        uint256 amount,
        bytes32 ivms101Hash
    ) external onlyRole(Roles.COMPLIANCE_OFFICER_ROLE) {
        bytes32 key = _travelRuleKey(from, to, amount);
        _s().travelRuleApproved[key] = true;
        emit TravelRuleRecorded(from, to, amount, ivms101Hash);
    }

    function _travelRuleKey(address from, address to, uint256 amount)
        private
        pure
        returns (bytes32)
    {
        return keccak256(abi.encodePacked(from, to, amount));
    }

    // ──────────────────────────────────────────────────────────────────────
    // Profil yönetimi
    // ──────────────────────────────────────────────────────────────────────

    function setProfile(address wallet, WalletProfile calldata profile)
        external
        onlyRole(Roles.KYC_WRITER_ROLE)
    {
        if (wallet == address(0)) revert Errors.ZeroAddress();
        _s().profiles[wallet] = profile;
        emit ProfileUpdated(wallet, profile.tier, profile.jurisdiction, profile.kycExpiresAt);
    }

    function getProfile(address wallet) external view returns (WalletProfile memory) {
        return _s().profiles[wallet];
    }

    // ──────────────────────────────────────────────────────────────────────
    // Dondurma
    // ──────────────────────────────────────────────────────────────────────

    function freeze(address wallet, bytes32 reasonHash)
        external
        onlyRole(Roles.COMPLIANCE_OFFICER_ROLE)
    {
        _s().profiles[wallet].frozen = true;
        emit Frozen(wallet, reasonHash);
    }

    function unfreeze(address wallet) external onlyRole(Roles.COMPLIANCE_OFFICER_ROLE) {
        _s().profiles[wallet].frozen = false;
        emit Unfrozen(wallet);
    }

    function setSanctioned(address wallet, bool value)
        external
        onlyRole(Roles.COMPLIANCE_OFFICER_ROLE)
    {
        _s().profiles[wallet].sanctioned = value;
        emit SanctionsUpdated(wallet, value);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Jurisdiction
    // ──────────────────────────────────────────────────────────────────────

    function setJurisdictionBlocked(bytes2 jurisdiction, bool blocked)
        external
        onlyRole(Roles.TREASURY_ROLE)
    {
        _s().jurisdictionBlocked[jurisdiction] = blocked;
        emit JurisdictionBlockUpdated(jurisdiction, blocked);
    }

    function isJurisdictionBlocked(bytes2 jurisdiction) external view returns (bool) {
        return _s().jurisdictionBlocked[jurisdiction];
    }

    // ──────────────────────────────────────────────────────────────────────
    // Travel Rule konfigürasyonu
    // ──────────────────────────────────────────────────────────────────────

    function setTravelRuleThreshold(uint256 thresholdWei) external onlyRole(Roles.TREASURY_ROLE) {
        _s().travelRuleThreshold = thresholdWei;
        emit TravelRuleThresholdUpdated(thresholdWei);
    }

    function travelRuleThreshold() external view returns (uint256) {
        return _s().travelRuleThreshold;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Görünüm yardımcıları
    // ──────────────────────────────────────────────────────────────────────

    function isKycValid(address wallet) external view returns (bool) {
        WalletProfile storage p = _s().profiles[wallet];
        return p.tier != KycTier.NONE && p.kycExpiresAt > block.timestamp;
    }

    function isFrozen(address wallet) external view returns (bool) {
        return _s().profiles[wallet].frozen;
    }

    function isSanctioned(address wallet) external view returns (bool) {
        return _s().profiles[wallet].sanctioned;
    }

    // ──────────────────────────────────────────────────────────────────────
    // UUPS
    // ──────────────────────────────────────────────────────────────────────

    function _authorizeUpgrade(address newImpl) internal override onlyRole(Roles.UPGRADER_ROLE) {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
    }
}
