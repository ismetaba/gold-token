// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { AccessControlUpgradeable } from
    "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from
    "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { EIP712Upgradeable } from
    "@openzeppelin/contracts-upgradeable/utils/cryptography/EIP712Upgradeable.sol";
import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";

import { IBurnController } from "./interfaces/IBurnController.sol";
import { IGoldToken } from "./interfaces/IGoldToken.sol";
import { IComplianceRegistry } from "./interfaces/IComplianceRegistry.sol";
import { Errors } from "./libraries/Errors.sol";
import { Roles } from "./libraries/Roles.sol";

/// @title BurnController
/// @notice User redemption burns + operator burns for GOLD (Ethereum mainnet).
///
/// Fee model: BURN_FEE_BPS = 25 (0.25%).
/// The full req.amount is burned on-chain.  A BurnFeeCollected event reports the
/// implied fee so the off-chain settlement system delivers req.amount - fee grams
/// (or equivalent cash) to the user.  This avoids any on-chain token transfer to
/// the treasury during the burn path while keeping the fee auditable on-chain.
contract BurnController is
    Initializable,
    AccessControlUpgradeable,
    ReentrancyGuardUpgradeable,
    UUPSUpgradeable,
    EIP712Upgradeable,
    IBurnController
{
    /// @notice Burn fee in basis points (25 bps = 0.25%).
    uint256 public constant BURN_FEE_BPS = 25;

    bytes32 private constant OPERATOR_BURN_TYPEHASH = keccak256(
        "OperatorBurn(address from,uint256 amount,bytes32 reasonHash,uint256 nonce,uint256 deadline)"
    );

    /// @custom:storage-location erc7201:gold.burn.storage
    struct BurnStorage {
        IGoldToken token;
        IComplianceRegistry compliance;
        uint256 minPhysicalGrams;   // minimum for PHYSICAL redemptions (default 100 * 1e18)
        mapping(bytes32 => RedemptionRequest) redemptions;
        mapping(bytes32 => bool) executed;
        mapping(bytes32 => uint256) executedAt;
        mapping(address => uint256) opBurnNonces;
    }

    // keccak256(abi.encode(uint256(keccak256("gold.burn.storage")) - 1)) & ~bytes32(uint256(0xff))
    bytes32 private constant STORAGE_LOCATION =
        0x8b62586ea3a733308049a7012d65db511d720e030d68785e3b52c81289a85900;

    function _s() private pure returns (BurnStorage storage $) {
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
        address token_,
        address compliance_,
        address operator,
        uint256 minPhysicalGrams_
    ) external initializer {
        if (
            treasury == address(0) || token_ == address(0) || compliance_ == address(0)
                || operator == address(0)
        ) revert Errors.ZeroAddress();

        __AccessControl_init();
        __ReentrancyGuard_init();
        __UUPSUpgradeable_init();
        __EIP712_init("GOLD BurnController", "1");

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);
        _grantRole(Roles.UPGRADER_ROLE, treasury);
        _grantRole(Roles.BURN_OPERATOR_ROLE, operator);

        BurnStorage storage $ = _s();
        $.token = IGoldToken(token_);
        $.compliance = IComplianceRegistry(compliance_);
        $.minPhysicalGrams = minPhysicalGrams_;
    }

    // ──────────────────────────────────────────────────────────────────────
    // User redemption burn
    // ──────────────────────────────────────────────────────────────────────

    function requestRedemption(RedemptionRequest calldata req)
        external
        nonReentrant
        onlyRole(Roles.BURN_OPERATOR_ROLE)
        returns (bytes32 reqId)
    {
        if (req.from == address(0)) revert Errors.ZeroAddress();
        if (req.amount == 0) revert Errors.ZeroAmount();

        BurnStorage storage $ = _s();

        if (req.redemptionType == RedemptionType.PHYSICAL && req.amount < $.minPhysicalGrams) {
            revert Errors.BelowMinimumRedemption(req.amount, $.minPhysicalGrams);
        }

        // Compliance check: user must not be frozen or sanctioned
        if (!$.compliance.canBurn(req.from, req.amount)) revert Errors.NotAuthorized();

        reqId = keccak256(
            abi.encode(req.from, req.amount, req.offChainOrderId, block.chainid, address(this))
        );

        if ($.executed[reqId]) revert Errors.ProposalAlreadyExecuted(reqId);

        $.redemptions[reqId] = req;
        $.executed[reqId] = true;
        $.executedAt[reqId] = block.timestamp;

        // Burn the full amount on-chain; off-chain settlement delivers (amount - fee)
        $.token.burnFrom(req.from, req.amount);

        // Compute and emit the burn fee for off-chain settlement
        uint256 fee = (req.amount * BURN_FEE_BPS) / 10_000;
        if (fee > 0) {
            emit BurnFeeCollected(reqId, req.from, fee);
        }

        emit RedemptionRequested(
            reqId, req.from, req.amount, req.redemptionType, req.offChainOrderId
        );
        emit RedemptionExecuted(reqId, $.token.totalSupply());
    }

    // ──────────────────────────────────────────────────────────────────────
    // Operator burn (operational correction with compliance officer signature)
    // ──────────────────────────────────────────────────────────────────────

    function operatorBurn(
        address from,
        uint256 amount,
        bytes32 reasonHash,
        uint256 deadline,
        bytes calldata complianceOfficerSig
    ) external nonReentrant onlyRole(Roles.BURN_OPERATOR_ROLE) {
        if (from == address(0)) revert Errors.ZeroAddress();
        if (amount == 0) revert Errors.ZeroAmount();
        if (block.timestamp > deadline) revert Errors.DeadlineExpired(deadline);

        BurnStorage storage $ = _s();
        uint256 nonce = $.opBurnNonces[from]++;

        bytes32 structHash = keccak256(
            abi.encode(OPERATOR_BURN_TYPEHASH, from, amount, reasonHash, nonce, deadline)
        );
        bytes32 digest = _hashTypedDataV4(structHash);
        address signer = ECDSA.recover(digest, complianceOfficerSig);

        // Dual-control: requires a valid Compliance Officer signature
        if (
            !IAccessControl(address($.compliance)).hasRole(
                Roles.COMPLIANCE_OFFICER_ROLE, signer
            )
        ) revert Errors.NotAuthorized();

        $.token.burnFrom(from, amount);
        emit OperatorBurn(from, amount, reasonHash);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Configuration
    // ──────────────────────────────────────────────────────────────────────

    function setMinPhysicalGrams(uint256 newMin) external onlyRole(Roles.TREASURY_ROLE) {
        _s().minPhysicalGrams = newMin;
        emit MinPhysicalGramsUpdated(newMin);
    }

    // ──────────────────────────────────────────────────────────────────────
    // View
    // ──────────────────────────────────────────────────────────────────────

    function minPhysicalGrams() external view returns (uint256) {
        return _s().minPhysicalGrams;
    }

    function getRedemption(bytes32 reqId)
        external
        view
        returns (RedemptionRequest memory, bool executed_, uint256 executedAt_)
    {
        BurnStorage storage $ = _s();
        return ($.redemptions[reqId], $.executed[reqId], $.executedAt[reqId]);
    }

    // ──────────────────────────────────────────────────────────────────────
    // EIP-712
    // ──────────────────────────────────────────────────────────────────────

    function DOMAIN_SEPARATOR() external view returns (bytes32) {
        return _domainSeparatorV4();
    }

    // ──────────────────────────────────────────────────────────────────────
    // UUPS
    // ──────────────────────────────────────────────────────────────────────

    function _authorizeUpgrade(address newImpl) internal override onlyRole(Roles.UPGRADER_ROLE) {
        if (newImpl == address(0)) revert Errors.ZeroAddress();
    }
}

interface IAccessControl {
    function hasRole(bytes32 role, address account) external view returns (bool);
}
