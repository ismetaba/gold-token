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
/// @notice Central compliance state for GOLD. GoldToken queries this registry in _update.
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
        // ── Upgrade timelock (appended at END to preserve storage layout) ──
        uint256 upgradeDelay;
        address scheduledImpl;
        uint256 scheduledAt;
        // ── Appended (preserve layout) ──
        // Eligibility snapshot for the scheduled upgrade, captured at schedule time so
        // that a later reduction of upgradeDelay cannot retroactively shorten the window.
        uint256 scheduledEligibleAt;
        // The GoldToken contract authorised to consume single-use Travel Rule approvals.
        address token;
    }

    /// @dev Default upgrade timelock: 48 hours.
    uint256 private constant DEFAULT_UPGRADE_DELAY = 48 hours;

    /// @dev Floor for the upgrade timelock so it can never be reduced to an unsafe value.
    uint256 private constant MIN_UPGRADE_DELAY = 24 hours;

    event UpgradeScheduled(address indexed newImpl, uint256 eligibleAt);
    event UpgradeCancelled(address indexed cancelledImpl);
    event UpgradeDelayUpdated(uint256 newDelay);
    // TokenUpdated and TravelRuleConsumed are declared in IComplianceRegistry.

    // keccak256(abi.encode(uint256(keccak256("gold.compliance.storage")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant STORAGE_LOCATION =
        0xa09d9d1de670bd1cae9af29c86455a7714e3aed8cd6dcbc4ed2dde5e98af2000;

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

        RegStorage storage $ = _s();
        $.travelRuleThreshold = travelRuleThreshold_;
        $.upgradeDelay = DEFAULT_UPGRADE_DELAY;
        emit TravelRuleThresholdUpdated(travelRuleThreshold_);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Primary gate — called by GoldToken._update
    // ──────────────────────────────────────────────────────────────────────

    function canTransfer(address from, address to, uint256 amount)
        external
        view
        returns (bool)
    {
        return _canTransfer(from, to, amount);
    }

    /// @notice State-changing transfer gate invoked by the token in _update.
    /// @dev Identical decision to canTransfer, but additionally CONSUMES a Travel Rule
    ///      approval when the transfer clears the gate. This makes each above-threshold
    ///      approval single-use (one IVMS-101 message authorises exactly one transfer),
    ///      as required by the FATF Travel Rule — a permanent per-(from,to,amount)
    ///      whitelist would let a single message authorise unlimited replays.
    ///      Restricted to the registered token so only genuine transfers consume approvals.
    function screenTransfer(address from, address to, uint256 amount)
        external
        returns (bool)
    {
        RegStorage storage $ = _s();
        if (msg.sender != $.token) revert Errors.NotAuthorized();
        if (!_canTransfer(from, to, amount)) return false;

        if (amount >= $.travelRuleThreshold) {
            bytes32 key = _travelRuleKey(from, to, amount);
            // The approval must exist (else _canTransfer would have returned false);
            // consume it so it cannot be replayed by a subsequent identical transfer.
            delete $.travelRuleApproved[key];
            emit TravelRuleConsumed(from, to, amount);
        }
        return true;
    }

    function _canTransfer(address from, address to, uint256 amount)
        private
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

        // Travel Rule: transfers above the threshold require a counterparty VASP message.
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
        // Gate on the recipient's KYC-verified jurisdiction, NOT the caller-supplied
        // provenance label: the proposer must not be able to mint into a wallet whose
        // verified jurisdiction is blocked by declaring an unblocked code. Also reject
        // when the declared provenance jurisdiction is itself blocked.
        if ($.jurisdictionBlocked[p.jurisdiction]) return false;
        if ($.jurisdictionBlocked[jurisdiction]) return false;
        if (p.tier == KycTier.NONE) return false;
        if (p.kycExpiresAt <= block.timestamp) return false;
        // No counterparty for mints — Travel Rule does not apply.
        return true;
    }

    function canBurn(address from, uint256) external view returns (bool) {
        RegStorage storage $ = _s();
        WalletProfile storage p = $.profiles[from];
        // A user may burn their own tokens unless frozen, sanctioned, or lacking valid KYC.
        if (p.frozen || p.sanctioned) return false;
        if (p.tier == KycTier.NONE) return false;
        if (p.kycExpiresAt <= block.timestamp) return false;
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
    // Profile management
    // ──────────────────────────────────────────────────────────────────────

    function setProfile(address wallet, WalletProfile calldata profile)
        external
        onlyRole(Roles.KYC_WRITER_ROLE)
    {
        if (wallet == address(0)) revert Errors.ZeroAddress();
        RegStorage storage $ = _s();
        WalletProfile storage existing = $.profiles[wallet];
        // Preserve the compliance-officer-controlled flags. setProfile is a KYC_WRITER
        // action (the automated KYC backend); it must not be able to clear a freeze or
        // sanctions mark set by the Compliance Officer via a full-struct overwrite.
        // Those flags are only mutable through freeze/unfreeze/setSanctioned.
        bool frozen = existing.frozen;
        bool sanctioned = existing.sanctioned;
        $.profiles[wallet] = profile;
        $.profiles[wallet].frozen = frozen;
        $.profiles[wallet].sanctioned = sanctioned;
        emit ProfileUpdated(wallet, profile.tier, profile.jurisdiction, profile.kycExpiresAt);
    }

    function getProfile(address wallet) external view returns (WalletProfile memory) {
        return _s().profiles[wallet];
    }

    // ──────────────────────────────────────────────────────────────────────
    // Freeze / unfreeze
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

    /// @notice Whether the wallet's stored (KYC-verified) jurisdiction is blocked.
    /// @dev Lets the token surface a jurisdiction-specific revert reason in _update.
    function isJurisdictionBlockedFor(address wallet) external view returns (bool) {
        RegStorage storage $ = _s();
        return $.jurisdictionBlocked[$.profiles[wallet].jurisdiction];
    }

    // ──────────────────────────────────────────────────────────────────────
    // Token registration (for single-use Travel Rule consumption)
    // ──────────────────────────────────────────────────────────────────────

    /// @notice Register the GoldToken authorised to consume Travel Rule approvals.
    /// @dev TREASURY_ROLE only. Must be set (post-deploy) before above-threshold
    ///      transfers can settle, since screenTransfer is restricted to this address.
    function setToken(address newToken) external onlyRole(Roles.TREASURY_ROLE) {
        if (newToken == address(0)) revert Errors.ZeroAddress();
        RegStorage storage $ = _s();
        address old = $.token;
        $.token = newToken;
        emit TokenUpdated(old, newToken);
    }

    function token() external view returns (address) {
        return _s().token;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Travel Rule configuration
    // ──────────────────────────────────────────────────────────────────────

    function setTravelRuleThreshold(uint256 thresholdWei) external onlyRole(Roles.TREASURY_ROLE) {
        _s().travelRuleThreshold = thresholdWei;
        emit TravelRuleThresholdUpdated(thresholdWei);
    }

    function travelRuleThreshold() external view returns (uint256) {
        return _s().travelRuleThreshold;
    }

    // ──────────────────────────────────────────────────────────────────────
    // View helpers
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

    function scheduleUpgrade(address newImpl) external onlyRole(Roles.UPGRADER_ROLE) {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
        RegStorage storage $ = _s();
        $.scheduledImpl = newImpl;
        $.scheduledAt = block.timestamp;
        // Snapshot the eligibility time NOW so a later setUpgradeDelay cannot shorten it.
        uint256 eligibleAt = block.timestamp + $.upgradeDelay;
        $.scheduledEligibleAt = eligibleAt;
        emit UpgradeScheduled(newImpl, eligibleAt);
    }

    function cancelScheduledUpgrade() external onlyRole(Roles.UPGRADER_ROLE) {
        RegStorage storage $ = _s();
        address cancelled = $.scheduledImpl;
        $.scheduledImpl = address(0);
        $.scheduledAt = 0;
        $.scheduledEligibleAt = 0;
        emit UpgradeCancelled(cancelled);
    }

    function setUpgradeDelay(uint256 newDelay) external onlyRole(Roles.TREASURY_ROLE) {
        // Enforce a floor so the timelock cannot be neutralised. Only affects upgrades
        // scheduled AFTER this call; already-scheduled upgrades keep their snapshot.
        if (newDelay < MIN_UPGRADE_DELAY) revert Errors.UpgradeDelayBelowMinimum(newDelay, MIN_UPGRADE_DELAY);
        _s().upgradeDelay = newDelay;
        emit UpgradeDelayUpdated(newDelay);
    }

    function upgradeDelay() external view returns (uint256) {
        return _s().upgradeDelay;
    }

    function scheduledUpgrade() external view returns (address impl, uint256 scheduledAt) {
        RegStorage storage $ = _s();
        return ($.scheduledImpl, $.scheduledAt);
    }

    /// @dev UPGRADER_ROLE + on-chain timelock: target must have been scheduled at least
    ///      `upgradeDelay` seconds earlier. Eligibility is read from the snapshot taken at
    ///      schedule time, so a subsequent change to upgradeDelay cannot shorten the window.
    function _authorizeUpgrade(address newImpl) internal override onlyRole(Roles.UPGRADER_ROLE) {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
        RegStorage storage $ = _s();
        if ($.scheduledImpl != newImpl || $.scheduledAt == 0) {
            revert Errors.UpgradeNotTimelocked();
        }
        if (block.timestamp < $.scheduledEligibleAt) {
            revert Errors.UpgradeTimelockActive($.scheduledEligibleAt);
        }
        $.scheduledImpl = address(0);
        $.scheduledAt = 0;
        $.scheduledEligibleAt = 0;
    }
}
