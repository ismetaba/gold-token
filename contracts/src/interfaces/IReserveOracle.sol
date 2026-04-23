// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IReserveOracle
/// @notice Aylık bağımsız denetim atestasyonlarının değişmez (append-only) kaydı.
/// @dev Bu kontrat UUPS DEĞİL — immutable deploy. Her ay denetçi (Big Four)
///      yeni atestasyon yayınlar. Geçmiş silinemez, düzenlenemez.
interface IReserveOracle {
    struct Attestation {
        uint64 timestamp;       // denetim tarihi (block.timestamp)
        uint64 asOf;            // denetimin referans aldığı gün (UTC midnight)
        uint256 totalGrams;     // tüm kasaların toplam altın miktarı (1e18 ile wei)
        bytes32 merkleRoot;     // çubuk seviyesinde Merkle ağaç kökü
        bytes32 ipfsCid;        // tam denetim paketinin IPFS CID'i (bytes32 encoded)
        address auditor;        // denetim firmasının on-chain adresi
    }

    /// @notice Yeni denetim atestasyonu yayınla. Sadece AUDITOR_ROLE.
    /// @dev timestamp ve asOf, önceki atestasyondan büyük olmalı (monotonic).
    function publish(Attestation calldata a, bytes calldata signature) external;

    /// @notice Son (en güncel) atestasyon.
    function latest() external view returns (Attestation memory);

    /// @notice Index ile atestasyon (append-only history).
    function attestationAt(uint256 index) external view returns (Attestation memory);

    /// @notice Toplam yayınlanmış atestasyon sayısı.
    function attestationCount() external view returns (uint256);

    /// @notice Son atestasyonun yayınlanmasından bu yana saniye.
    function timeSinceLatest() external view returns (uint256);

    /// @notice Son atestasyonun toplam altın miktarı (uyum kontrolü için).
    function latestAttestedGrams() external view returns (uint256);

    /// @notice Bir çubuğun belirli bir atestasyonda dahil olduğunu Merkle proof ile kanıtla.
    /// @param barLeaf keccak256(abi.encode(serial, weight, purity, vault, refinerId))
    function verifyBarInclusion(
        uint256 attestationIndex,
        bytes32 barLeaf,
        bytes32[] calldata proof
    ) external view returns (bool);

    /// @notice Yetkili denetçi firmaları (EIP-712 imzalarında).
    function isAuthorizedAuditor(address auditor) external view returns (bool);

    event AttestationPublished(
        uint256 indexed index,
        uint64 timestamp,
        uint64 asOf,
        uint256 totalGrams,
        bytes32 merkleRoot,
        bytes32 ipfsCid,
        address indexed auditor
    );
    event AuditorAuthorized(address indexed auditor);
    event AuditorDeauthorized(address indexed auditor);
}
