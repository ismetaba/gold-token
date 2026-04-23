// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IReserveOracle
/// @notice Immutable (append-only) record of monthly independent audit attestations.
/// @dev This contract is NOT UUPS — immutable deploy. A new version is deployed for
///      bug fixes or role changes, and the MintController is updated via setOracle.
///      Audit history cannot be deleted or altered.
interface IReserveOracle {
    struct Attestation {
        uint64 timestamp;       // publication time (block.timestamp)
        uint64 asOf;            // reference date of the audit (UTC midnight)
        uint256 totalGrams;     // total gold across all vaults (wei; 1 gram = 1e18)
        bytes32 merkleRoot;     // Merkle root of the per-bar leaf set
        bytes32 ipfsCid;        // IPFS CID of the full audit package (bytes32-encoded)
        address auditor;        // on-chain address of the auditing firm
    }

    /// @notice Publish a new audit attestation. AUDITOR_ROLE only.
    /// @dev timestamp and asOf must be strictly greater than the previous attestation
    ///      (monotonic).
    function publish(Attestation calldata a, bytes calldata signature) external;

    /// @notice Most recent (latest) attestation.
    function latest() external view returns (Attestation memory);

    /// @notice Attestation by index (append-only history).
    function attestationAt(uint256 index) external view returns (Attestation memory);

    /// @notice Total number of published attestations.
    function attestationCount() external view returns (uint256);

    /// @notice Seconds elapsed since the latest attestation was published.
    function timeSinceLatest() external view returns (uint256);

    /// @notice Total gold grams from the latest attestation (used for reserve invariant).
    function latestAttestedGrams() external view returns (uint256);

    /// @notice Verify that a bar is included in a specific attestation via Merkle proof.
    /// @param barLeaf keccak256(abi.encode(serial, weight, purity, vault, refinerId))
    function verifyBarInclusion(
        uint256 attestationIndex,
        bytes32 barLeaf,
        bytes32[] calldata proof
    ) external view returns (bool);

    /// @notice Whether the given address is an authorised auditor.
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
