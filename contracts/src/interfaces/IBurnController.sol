// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IBurnController
/// @notice İki yoldan yakım:
///         1. Kullanıcı itfa (redeem) — kullanıcı yakar, off-chain kasa serbest bırakır
///         2. Operatör burn — ops düzeltme, sadece BURN_OPERATOR_ROLE
interface IBurnController {
    enum RedemptionType {
        CASH_BACK,      // fiat karşılığı itfa (TRY/USD/EUR/AED)
        PHYSICAL        // fiziksel altın teslim (min 1kg)
    }

    struct RedemptionRequest {
        address from;
        uint256 amount;             // gram wei
        RedemptionType redemptionType;
        bytes32 offChainOrderId;    // Backend order ID (UUID)
        string deliveryRef;         // IBAN/adres hash vb. (off-chain referans)
    }

    /// @notice Kullanıcı itfa yakımı — approve + burn.
    /// @dev Kullanıcı önce GoldToken.approve(burnController, amount) yapar,
    ///      sonra backend bu fonksiyonu çağırır (MINT_PROPOSER gibi servis).
    ///      Alternatif: kullanıcı doğrudan imzalı meta-tx ile çağırabilir (EIP-2771).
    function requestRedemption(RedemptionRequest calldata req) external returns (bytes32 reqId);

    /// @notice Fiziksel itfa için min tutar kontrolü.
    function minPhysicalGrams() external view returns (uint256);

    /// @notice Operatör burn — ops düzeltmesi (örn. yanlış mint geri alımı).
    ///         Sadece BURN_OPERATOR_ROLE + COMPLIANCE_OFFICER onayı gerekir.
    function operatorBurn(
        address from,
        uint256 amount,
        bytes32 reasonHash,
        bytes calldata complianceOfficerSig
    ) external;

    // Görünüm
    function getRedemption(bytes32 reqId)
        external
        view
        returns (RedemptionRequest memory, bool executed, uint256 executedAt);

    event RedemptionRequested(
        bytes32 indexed reqId,
        address indexed from,
        uint256 amount,
        RedemptionType redemptionType,
        bytes32 offChainOrderId
    );
    event RedemptionExecuted(bytes32 indexed reqId, uint256 newTotalSupply);
    event OperatorBurn(address indexed from, uint256 amount, bytes32 reasonHash);
    event MinPhysicalGramsUpdated(uint256 newMin);
}
