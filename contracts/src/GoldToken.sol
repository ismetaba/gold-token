// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { ERC20Upgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import { ERC20PermitUpgradeable } from
    "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20PermitUpgradeable.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { AccessControlUpgradeable } from
    "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

import { IERC20Permit } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Permit.sol";

import { IGoldToken } from "./interfaces/IGoldToken.sol";
import { IComplianceRegistry } from "./interfaces/IComplianceRegistry.sol";
import { Errors } from "./libraries/Errors.sol";
import { Roles } from "./libraries/Roles.sol";

/// @title GoldToken
/// @notice GOLD — 1 token = 1 gram of 99.99% physical gold. 18 decimals. Ethereum mainnet.
/// @dev UUPS-upgradeable. All transfers are routed through ComplianceRegistry.canTransfer().
///      Mint and burn are restricted to the authorised controller addresses.
///      On-chain transfer fee: 0% (zero). Mint and burn fees are handled by the controllers.
contract GoldToken is
    Initializable,
    ERC20Upgradeable,
    ERC20PermitUpgradeable,
    PausableUpgradeable,
    AccessControlUpgradeable,
    UUPSUpgradeable,
    IGoldToken
{
    /// @custom:storage-location erc7201:gold.token.storage
    struct GoldTokenStorage {
        address complianceRegistry;
        address mintController;
        address burnController;
        // ── Upgrade timelock (appended at END to preserve storage layout) ──
        uint256 upgradeDelay;           // required delay between schedule and execute (seconds)
        address scheduledImpl;          // implementation scheduled for upgrade
        uint256 scheduledAt;            // timestamp the scheduled impl became eligible (schedule time)
        // ── Operator-burn pause bypass flag ──
        // Set only for the duration of a single operatorBurnFrom() call so that the
        // compliance clawback can execute the burn even while the token is paused.
        bool operatorBurnInProgress;
        // Eligibility snapshot for the scheduled upgrade, captured at schedule time so a
        // later reduction of upgradeDelay cannot retroactively shorten the window.
        uint256 scheduledEligibleAt;
    }

    /// @dev Default upgrade timelock: 48 hours.
    uint256 private constant DEFAULT_UPGRADE_DELAY = 48 hours;

    /// @dev Floor for the upgrade timelock so it can never be reduced to an unsafe value.
    uint256 private constant MIN_UPGRADE_DELAY = 24 hours;

    // keccak256(abi.encode(uint256(keccak256("gold.token.storage")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant STORAGE_LOCATION =
        0x4964a4de2330c7e38a2bb854e4da584e39f4533666e0f28100a6fba722473300;

    function _getStorage() private pure returns (GoldTokenStorage storage $) {
        assembly {
            $.slot := STORAGE_LOCATION
        }
    }

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(
        string memory name_,
        string memory symbol_,
        address treasury,
        address pauser,
        address complianceRegistry_
    ) external initializer {
        if (
            treasury == address(0) || pauser == address(0)
                || complianceRegistry_ == address(0)
        ) revert Errors.ZeroAddress();

        __ERC20_init(name_, symbol_);
        __ERC20Permit_init(name_);
        __Pausable_init();
        __AccessControl_init();
        __UUPSUpgradeable_init();

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);
        _grantRole(Roles.UPGRADER_ROLE, treasury);
        _grantRole(Roles.PAUSER_ROLE, pauser);

        GoldTokenStorage storage $ = _getStorage();
        $.complianceRegistry = complianceRegistry_;
        $.upgradeDelay = DEFAULT_UPGRADE_DELAY;
    }

    // ──────────────────────────────────────────────────────────────────────
    // ERC-20
    // ──────────────────────────────────────────────────────────────────────

    /// @dev OZ v5: the _update hook covers all transfer, mint, and burn paths.
    ///      Pause is enforced here (rather than via the whenNotPaused modifier) so that
    ///      the emergency compliance clawback (operatorBurnFrom) can bypass the pause gate
    ///      while still being subject to all other invariants.
    function _update(address from, address to, uint256 value)
        internal
        override(ERC20Upgradeable)
    {
        GoldTokenStorage storage $ = _getStorage();

        // Pause gate — bypassed only during an in-progress operator (clawback) burn.
        if (paused() && !$.operatorBurnInProgress) {
            revert EnforcedPause();
        }

        if (from != address(0) && to != address(0)) {
            // Transfer: compliance gate. screenTransfer applies the same rules as the
            // canTransfer view but additionally consumes any single-use Travel Rule
            // approval so it cannot be replayed by a later identical transfer.
            IComplianceRegistry reg = IComplianceRegistry($.complianceRegistry);
            if (!reg.screenTransfer(from, to, value)) {
                // Return a specific error describing which rule was triggered
                if (reg.isFrozen(from)) revert Errors.WalletFrozen(from);
                if (reg.isFrozen(to)) revert Errors.WalletFrozen(to);
                if (reg.isSanctioned(from)) revert Errors.SanctionsHit(from);
                if (reg.isSanctioned(to)) revert Errors.SanctionsHit(to);
                if (!reg.isKycValid(from)) revert Errors.KycRequired(from);
                if (!reg.isKycValid(to)) revert Errors.KycRequired(to);
                if (reg.isJurisdictionBlockedFor(from)) {
                    revert Errors.JurisdictionBlocked(reg.getProfile(from).jurisdiction);
                }
                if (reg.isJurisdictionBlockedFor(to)) {
                    revert Errors.JurisdictionBlocked(reg.getProfile(to).jurisdiction);
                }
                if (reg.travelRuleRequired(from, to, value)) {
                    revert Errors.TravelRuleRequired(from, to, value);
                }
                revert Errors.NotAuthorized();
            }
        }

        super._update(from, to, value);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Mint / Burn (controllers only)
    // ──────────────────────────────────────────────────────────────────────

    function mint(address to, uint256 amount, bytes2 jurisdiction) external {
        GoldTokenStorage storage $ = _getStorage();
        if (msg.sender != $.mintController) revert Errors.NotAuthorized();
        if (to == address(0)) revert Errors.ZeroAddress();
        if (amount == 0) revert Errors.ZeroAmount();

        _mint(to, amount);
        emit Minted(to, amount, jurisdiction);
    }

    function burnFrom(address from, uint256 amount) external {
        GoldTokenStorage storage $ = _getStorage();
        if (msg.sender != $.burnController) revert Errors.NotAuthorized();
        if (amount == 0) revert Errors.ZeroAmount();

        // Pull-burn: user must have approved the burn controller
        _spendAllowance(from, msg.sender, amount);
        _burn(from, amount);
        emit Burned(from, amount);
    }

    /// @notice Emergency compliance clawback burn, callable only by the burn controller.
    /// @dev This path exists to support BurnController.operatorBurn, an emergency
    ///      compliance/clawback tool (e.g. reversing a sanctioned/frozen wallet's balance
    ///      under dual control). Unlike burnFrom it:
    ///        - does NOT require an ERC-20 allowance (the target is non-cooperative), and
    ///        - executes even while the token is PAUSED, because freezing all transfers
    ///          must not also block the compliance remediation that pause is protecting.
    ///      All compliance-policy gating for this path lives in BurnController.operatorBurn
    ///      (dual-control compliance-officer signature + clawback eligibility check).
    function operatorBurnFrom(address from, uint256 amount) external {
        GoldTokenStorage storage $ = _getStorage();
        if (msg.sender != $.burnController) revert Errors.NotAuthorized();
        if (amount == 0) revert Errors.ZeroAmount();

        $.operatorBurnInProgress = true;
        _burn(from, amount);
        $.operatorBurnInProgress = false;

        emit Burned(from, amount);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Pausable
    // ──────────────────────────────────────────────────────────────────────

    function pause() external onlyRole(Roles.PAUSER_ROLE) {
        _pause();
    }

    function unpause() external onlyRole(Roles.TREASURY_ROLE) {
        _unpause();
    }

    // ──────────────────────────────────────────────────────────────────────
    // Administration
    // ──────────────────────────────────────────────────────────────────────

    function setComplianceRegistry(address newRegistry) external onlyRole(Roles.TREASURY_ROLE) {
        if (newRegistry == address(0)) revert Errors.ZeroAddress();
        GoldTokenStorage storage $ = _getStorage();
        address old = $.complianceRegistry;
        $.complianceRegistry = newRegistry;
        emit ComplianceRegistryUpdated(old, newRegistry);
    }

    function setMintController(address newController) external onlyRole(Roles.TREASURY_ROLE) {
        if (newController == address(0)) revert Errors.ZeroAddress();
        GoldTokenStorage storage $ = _getStorage();
        address old = $.mintController;
        $.mintController = newController;
        emit MintControllerUpdated(old, newController);
    }

    function setBurnController(address newController) external onlyRole(Roles.TREASURY_ROLE) {
        if (newController == address(0)) revert Errors.ZeroAddress();
        GoldTokenStorage storage $ = _getStorage();
        address old = $.burnController;
        $.burnController = newController;
        emit BurnControllerUpdated(old, newController);
    }

    // ──────────────────────────────────────────────────────────────────────
    // View
    // ──────────────────────────────────────────────────────────────────────

    function complianceRegistry() external view returns (address) {
        return _getStorage().complianceRegistry;
    }

    function mintController() external view returns (address) {
        return _getStorage().mintController;
    }

    function burnController() external view returns (address) {
        return _getStorage().burnController;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Upgrade (UUPS)
    // ──────────────────────────────────────────────────────────────────────

    /// @notice Schedule an implementation for upgrade. UPGRADER_ROLE only.
    /// @dev The scheduled implementation may only be applied (via upgradeToAndCall)
    ///      after `upgradeDelay` seconds have elapsed. Scheduling a new implementation
    ///      overwrites any previously scheduled one and resets the timer.
    function scheduleUpgrade(address newImpl) external onlyRole(Roles.UPGRADER_ROLE) {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
        GoldTokenStorage storage $ = _getStorage();
        $.scheduledImpl = newImpl;
        $.scheduledAt = block.timestamp;
        // Snapshot the eligibility time NOW so a later setUpgradeDelay cannot shorten it.
        uint256 eligibleAt = block.timestamp + $.upgradeDelay;
        $.scheduledEligibleAt = eligibleAt;
        emit UpgradeScheduled(newImpl, eligibleAt);
    }

    /// @notice Cancel a previously scheduled upgrade. UPGRADER_ROLE only.
    function cancelScheduledUpgrade() external onlyRole(Roles.UPGRADER_ROLE) {
        GoldTokenStorage storage $ = _getStorage();
        address cancelled = $.scheduledImpl;
        $.scheduledImpl = address(0);
        $.scheduledAt = 0;
        $.scheduledEligibleAt = 0;
        emit UpgradeCancelled(cancelled);
    }

    /// @notice Update the upgrade timelock delay (seconds). TREASURY_ROLE only.
    /// @dev Enforces a MIN_UPGRADE_DELAY floor so the timelock cannot be neutralised.
    ///      Only affects upgrades scheduled after this call; already-scheduled upgrades
    ///      keep the eligibility snapshot taken at schedule time.
    function setUpgradeDelay(uint256 newDelay) external onlyRole(Roles.TREASURY_ROLE) {
        if (newDelay < MIN_UPGRADE_DELAY) {
            revert Errors.UpgradeDelayBelowMinimum(newDelay, MIN_UPGRADE_DELAY);
        }
        _getStorage().upgradeDelay = newDelay;
        emit UpgradeDelayUpdated(newDelay);
    }

    function upgradeDelay() external view returns (uint256) {
        return _getStorage().upgradeDelay;
    }

    function scheduledUpgrade() external view returns (address impl, uint256 scheduledAt) {
        GoldTokenStorage storage $ = _getStorage();
        return ($.scheduledImpl, $.scheduledAt);
    }

    /// @dev Enforces both the UPGRADER_ROLE and the on-chain upgrade timelock. The target
    ///      implementation must have been scheduled via scheduleUpgrade() at least
    ///      `upgradeDelay` seconds earlier. Once applied, the schedule slot is cleared so
    ///      the same window cannot be reused for a different (unscheduled) implementation.
    function _authorizeUpgrade(address newImpl)
        internal
        override
        onlyRole(Roles.UPGRADER_ROLE)
    {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
        GoldTokenStorage storage $ = _getStorage();
        if ($.scheduledImpl != newImpl || $.scheduledAt == 0) {
            revert Errors.UpgradeNotTimelocked();
        }
        // Eligibility is read from the snapshot taken at schedule time, so a subsequent
        // reduction of upgradeDelay cannot shorten an already-scheduled window.
        if (block.timestamp < $.scheduledEligibleAt) {
            revert Errors.UpgradeTimelockActive($.scheduledEligibleAt);
        }
        // Consume the schedule so it cannot be replayed.
        $.scheduledImpl = address(0);
        $.scheduledAt = 0;
        $.scheduledEligibleAt = 0;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Multiple-inheritance overrides
    // ──────────────────────────────────────────────────────────────────────

    function nonces(address owner)
        public
        view
        override(ERC20PermitUpgradeable, IERC20Permit)
        returns (uint256)
    {
        return super.nonces(owner);
    }

    function paused()
        public
        view
        override(PausableUpgradeable, IGoldToken)
        returns (bool)
    {
        return super.paused();
    }

    // ──────────────────────────────────────────────────────────────────────
    // ERC-165
    // ──────────────────────────────────────────────────────────────────────

    function supportsInterface(bytes4 interfaceId)
        public
        view
        override(AccessControlUpgradeable)
        returns (bool)
    {
        return interfaceId == type(IGoldToken).interfaceId
            || super.supportsInterface(interfaceId);
    }
}
