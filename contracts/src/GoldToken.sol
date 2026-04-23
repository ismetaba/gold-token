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
/// @notice GOLD — 1 token = 1 gram %99.99 altın. 18 decimals.
/// @dev UUPS upgradable. Tüm transferler ComplianceRegistry.canTransfer()'dan geçer.
///      Mint/burn sadece yetkili controller adresleri üzerinden.
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
    }

    // keccak256(abi.encode(uint256(keccak256("gold.token.storage")) - 1)) & ~bytes32(uint256(0xff))
    // TODO(deploy): precompute ve hardcode et — aşağıdaki değer placeholder.
    bytes32 private constant STORAGE_LOCATION =
        0xa1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a100;

    function _getStorage() private pure returns (GoldTokenStorage storage $) {
        assembly {
            $.slot := STORAGE_LOCATION
        }
    }

    /// @dev Constructor replaced by initializer for upgradability.
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

        _getStorage().complianceRegistry = complianceRegistry_;
    }

    // ──────────────────────────────────────────────────────────────────────
    // ERC-20
    // ──────────────────────────────────────────────────────────────────────

    /// @dev OZ v5: _update hook'u tüm transfer, mint, burn'ü kapsar.
    function _update(address from, address to, uint256 value)
        internal
        override(ERC20Upgradeable)
        whenNotPaused
    {
        GoldTokenStorage storage $ = _getStorage();

        if (from != address(0) && to != address(0)) {
            // Transfer: compliance kapısı
            IComplianceRegistry reg = IComplianceRegistry($.complianceRegistry);
            if (!reg.canTransfer(from, to, value)) {
                // Hangi kurala takıldığını spesifik hata ile döndür
                if (reg.isFrozen(from)) revert Errors.WalletFrozen(from);
                if (reg.isFrozen(to)) revert Errors.WalletFrozen(to);
                if (reg.isSanctioned(from)) revert Errors.SanctionsHit(from);
                if (reg.isSanctioned(to)) revert Errors.SanctionsHit(to);
                if (!reg.isKycValid(from)) revert Errors.KycRequired(from);
                if (!reg.isKycValid(to)) revert Errors.KycRequired(to);
                if (reg.travelRuleRequired(from, to, value)) {
                    revert Errors.TravelRuleRequired(from, to, value);
                }
                // generic fallback
                revert Errors.NotAuthorized();
            }
        }

        super._update(from, to, value);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Mint / Burn (sadece controller'lar)
    // ──────────────────────────────────────────────────────────────────────

    function mint(address to, uint256 amount, bytes2 jurisdiction) external {
        GoldTokenStorage storage $ = _getStorage();
        if (msg.sender != $.mintController) revert Errors.NotAuthorized();
        if (to == address(0)) revert Errors.ZeroAddress();
        if (amount == 0) revert Errors.ZeroAmount();

        // Mint için alıcı tarafı canMint kontrolü controller'da yapılır (daha ucuz).
        // Burada sadece pause kontrolü _update içinde işler.
        _mint(to, amount);
        emit Minted(to, amount, jurisdiction);
    }

    function burnFrom(address from, uint256 amount) external {
        GoldTokenStorage storage $ = _getStorage();
        if (msg.sender != $.burnController) revert Errors.NotAuthorized();
        if (amount == 0) revert Errors.ZeroAmount();

        // allowance kontrolü (burn controller pull-burn yapar)
        _spendAllowance(from, msg.sender, amount);
        _burn(from, amount);
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
    // Yönetim
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
    // Görünüm
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

    /// @dev Timelock Treasury Safe tarafında zorlanır; burada sadece yetki.
    function _authorizeUpgrade(address newImpl)
        internal
        override
        onlyRole(Roles.UPGRADER_ROLE)
    {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
    }

    // ──────────────────────────────────────────────────────────────────────
    // Override çözümlemeleri (çoklu kalıtım)
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
