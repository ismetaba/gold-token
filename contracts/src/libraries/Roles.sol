// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

library Roles {
    /// @dev Treasury Safe (3/5). Kontrat sahipliği, parametre değişiklikleri, acil durum.
    bytes32 internal constant TREASURY_ROLE = keccak256("gold.role.treasury");

    /// @dev Upgrade yapabilen rol. Timelock + Treasury onayı gerekir.
    bytes32 internal constant UPGRADER_ROLE = keccak256("gold.role.upgrader");

    /// @dev Acil durumda pause edebilen rol. Sadece pause; unpause Treasury'dedir.
    bytes32 internal constant PAUSER_ROLE = keccak256("gold.role.pauser");

    /// @dev KYC Service backend. Profil yazma yetkisi.
    bytes32 internal constant KYC_WRITER_ROLE = keccak256("gold.role.kyc_writer");

    /// @dev Compliance Officer. Freeze/unfreeze, manuel inceleme.
    bytes32 internal constant COMPLIANCE_OFFICER_ROLE = keccak256("gold.role.compliance_officer");

    /// @dev Mint teklifi açabilir (Mint/Burn Service backend).
    bytes32 internal constant MINT_PROPOSER_ROLE = keccak256("gold.role.mint_proposer");

    /// @dev Mint teklifini onaylayabilir (5 adet; 3'ü gerekli).
    bytes32 internal constant MINT_APPROVER_ROLE = keccak256("gold.role.mint_approver");

    /// @dev Mint teklifini çalıştırabilir (tek rol, onay eşiği sağlandıktan sonra).
    bytes32 internal constant MINT_EXECUTOR_ROLE = keccak256("gold.role.mint_executor");

    /// @dev Operatör burn (itfa sonrası kasadan düşüş).
    bytes32 internal constant BURN_OPERATOR_ROLE = keccak256("gold.role.burn_operator");

    /// @dev Denetim atestasyonu yayınlayabilir (Big Four firma cüzdanı).
    bytes32 internal constant AUDITOR_ROLE = keccak256("gold.role.auditor");
}
