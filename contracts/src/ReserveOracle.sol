// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";
import { EIP712 } from "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import { MerkleProof } from "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";

import { IReserveOracle } from "./interfaces/IReserveOracle.sol";
import { Errors } from "./libraries/Errors.sol";
import { Roles } from "./libraries/Roles.sol";

/// @title ReserveOracle
/// @notice Immutable (append-only) log of monthly independent audit attestations.
/// @dev This contract is NOT UUPS — it is an immutable deploy. The immutability of
///      the audit history is the cornerstone of reserve trust. For bug fixes or role
///      changes a new version is deployed and MintController.setOracle is updated.
contract ReserveOracle is AccessControl, EIP712, IReserveOracle {
    bytes32 private constant ATTESTATION_TYPEHASH = keccak256(
        "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
    );

    Attestation[] private _attestations;

    constructor(address treasury, address[] memory initialAuditors)
        EIP712("GOLD ReserveOracle", "1")
    {
        if (treasury == address(0)) revert Errors.ZeroAddress();

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);

        for (uint256 i = 0; i < initialAuditors.length; i++) {
            address a = initialAuditors[i];
            if (a == address(0)) revert Errors.ZeroAddress();
            _grantRole(Roles.AUDITOR_ROLE, a);
            emit AuditorAuthorized(a);
        }
    }

    // ──────────────────────────────────────────────────────────────────────
    // Publish (append-only)
    // ──────────────────────────────────────────────────────────────────────

    function publish(Attestation calldata a, bytes calldata signature) external {
        if (!hasRole(Roles.AUDITOR_ROLE, a.auditor)) revert Errors.UnknownAuditor(a.auditor);

        // EIP-712 signature verification
        bytes32 structHash = keccak256(
            abi.encode(
                ATTESTATION_TYPEHASH,
                a.timestamp,
                a.asOf,
                a.totalGrams,
                a.merkleRoot,
                a.ipfsCid,
                a.auditor
            )
        );
        bytes32 digest = _hashTypedDataV4(structHash);
        address recovered = ECDSA.recover(digest, signature);
        if (recovered != a.auditor) revert Errors.InvalidAuditorSignature();

        // Monotonicity: timestamps must strictly increase
        uint256 n = _attestations.length;
        if (n > 0) {
            Attestation storage prev = _attestations[n - 1];
            if (a.timestamp <= prev.timestamp || a.asOf <= prev.asOf) {
                revert Errors.AttestationMonotonicityViolated(prev.timestamp, a.timestamp);
            }
        }

        // Sanity check: attestation must not claim a future timestamp (±1 hour tolerance)
        if (a.timestamp > block.timestamp + 1 hours) {
            revert Errors.AttestationMonotonicityViolated(uint64(block.timestamp), a.timestamp);
        }

        _attestations.push(a);

        emit AttestationPublished(
            n, a.timestamp, a.asOf, a.totalGrams, a.merkleRoot, a.ipfsCid, a.auditor
        );
    }

    // ──────────────────────────────────────────────────────────────────────
    // Queries
    // ──────────────────────────────────────────────────────────────────────

    function latest() external view returns (Attestation memory) {
        uint256 n = _attestations.length;
        if (n == 0) {
            // No attestations yet; caller (MintController) catches this as a staleness failure.
            return Attestation(0, 0, 0, bytes32(0), bytes32(0), address(0));
        }
        return _attestations[n - 1];
    }

    function attestationAt(uint256 index) external view returns (Attestation memory) {
        return _attestations[index];
    }

    function attestationCount() external view returns (uint256) {
        return _attestations.length;
    }

    function timeSinceLatest() external view returns (uint256) {
        uint256 n = _attestations.length;
        if (n == 0) return type(uint256).max; // effectively stale
        uint64 ts = _attestations[n - 1].timestamp;
        return block.timestamp > ts ? block.timestamp - ts : 0;
    }

    function latestAttestedGrams() external view returns (uint256) {
        uint256 n = _attestations.length;
        if (n == 0) return 0;
        return _attestations[n - 1].totalGrams;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Bar verification
    // ──────────────────────────────────────────────────────────────────────

    function verifyBarInclusion(
        uint256 attestationIndex,
        bytes32 barLeaf,
        bytes32[] calldata proof
    ) external view returns (bool) {
        bytes32 root = _attestations[attestationIndex].merkleRoot;
        return MerkleProof.verifyCalldata(proof, root, barLeaf);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Auditor management
    // ──────────────────────────────────────────────────────────────────────

    function isAuthorizedAuditor(address auditor) external view returns (bool) {
        return hasRole(Roles.AUDITOR_ROLE, auditor);
    }

    function authorizeAuditor(address auditor) external onlyRole(Roles.TREASURY_ROLE) {
        if (auditor == address(0)) revert Errors.ZeroAddress();
        _grantRole(Roles.AUDITOR_ROLE, auditor);
        emit AuditorAuthorized(auditor);
    }

    function deauthorizeAuditor(address auditor) external onlyRole(Roles.TREASURY_ROLE) {
        _revokeRole(Roles.AUDITOR_ROLE, auditor);
        emit AuditorDeauthorized(auditor);
    }

    function DOMAIN_SEPARATOR() external view returns (bytes32) {
        return _domainSeparatorV4();
    }
}
