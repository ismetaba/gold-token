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
/// @notice Append-only log of monthly independent audit attestations.
/// @dev This contract is NOT UUPS. The ATTESTATION HISTORY is genuinely immutable:
///      attestations are only ever appended, never overwritten or deleted, and that
///      history is the cornerstone of reserve trust. The GOVERNANCE PARAMETERS that
///      gate acceptance of new attestations — the auditor set, signature threshold,
///      max growth cap, and minimum attestation interval — remain mutable by
///      TREASURY_ROLE (see authorizeAuditor/deauthorizeAuditor, setSignatureThreshold,
///      setMaxGrowthBps, setMinAttestationInterval). To migrate to a materially
///      different oracle, deploy a new version and point the consumer at it via
///      MintController.setOracle.
contract ReserveOracle is AccessControl, EIP712, IReserveOracle {
    bytes32 private constant ATTESTATION_TYPEHASH = keccak256(
        "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
    );

    Attestation[] private _attestations;

    // ── Appended storage (this contract is an immutable, non-upgradeable deploy;
    //    fields are appended at the END to keep the layout simple and review-friendly) ──

    /// @dev Number of distinct auditor signatures required to publish an attestation.
    uint256 private _signatureThreshold;

    /// @dev Number of currently authorised auditors (kept in sync via role hooks).
    uint256 private _auditorCount;

    /// @dev Maximum allowed growth in totalGrams between consecutive attestations,
    ///      in basis points. 10_000 bps = +100%. Default 5_000 bps (+50%).
    uint256 private _maxGrowthBps;

    /// @dev Minimum wall-clock spacing (seconds) between consecutive attestations. This
    ///      prevents the per-step growth cap from being compounded by chaining many
    ///      attestations within a single block/transaction (each incrementing the
    ///      supplied timestamp by 1). Because a.timestamp is bounded to block.timestamp+1h,
    ///      any interval > 1h makes same-block batching infeasible.
    uint256 private _minAttestationInterval;

    /// @dev Default signature threshold when at least this many auditors are configured.
    uint256 private constant DEFAULT_SIGNATURE_THRESHOLD = 2;

    /// @dev Default max growth between attestations: +50%.
    uint256 private constant DEFAULT_MAX_GROWTH_BPS = 5_000;

    /// @dev Default minimum spacing between attestations: 1 hour (anti-batching floor).
    ///      Operators should raise this toward the real audit cadence (e.g. weeks).
    uint256 private constant DEFAULT_MIN_ATTESTATION_INTERVAL = 1 hours;

    /// @dev Upper bound for the growth cap so an extreme value cannot cause prevGrams*bps
    ///      to overflow/DoS publish or make the cap meaningless.
    uint256 private constant MAX_GROWTH_BPS_CAP = 100_000; // +1000%

    constructor(address treasury, address[] memory initialAuditors)
        EIP712("GOLD ReserveOracle", "1")
    {
        if (treasury == address(0)) revert Errors.ZeroAddress();

        _grantRole(DEFAULT_ADMIN_ROLE, treasury);
        _grantRole(Roles.TREASURY_ROLE, treasury);

        for (uint256 i = 0; i < initialAuditors.length; i++) {
            address a = initialAuditors[i];
            if (a == address(0)) revert Errors.ZeroAddress();
            // _grantRole increments _auditorCount via the role hook below.
            _grantRole(Roles.AUDITOR_ROLE, a);
            emit AuditorAuthorized(a);
        }

        // Default threshold is 2, clamped to the number of auditors so the contract is
        // always publishable (e.g. a single-auditor deploy uses threshold 1).
        uint256 defaultThreshold =
            _auditorCount < DEFAULT_SIGNATURE_THRESHOLD ? _auditorCount : DEFAULT_SIGNATURE_THRESHOLD;
        if (defaultThreshold == 0) defaultThreshold = 1; // never 0; enforced again on publish
        _signatureThreshold = defaultThreshold;
        emit SignatureThresholdUpdated(defaultThreshold);

        _maxGrowthBps = DEFAULT_MAX_GROWTH_BPS;
        emit MaxGrowthBpsUpdated(DEFAULT_MAX_GROWTH_BPS);

        _minAttestationInterval = DEFAULT_MIN_ATTESTATION_INTERVAL;
        emit MinAttestationIntervalUpdated(DEFAULT_MIN_ATTESTATION_INTERVAL);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Publish (append-only)
    // ──────────────────────────────────────────────────────────────────────

    /// @notice Publish a new attestation requiring an N-of-M auditor signature threshold.
    /// @dev Each signature must come from a distinct authorised auditor over the same
    ///      EIP-712 typed-data digest. The `a.auditor` field records the lead/submitting
    ///      auditor and must itself be one of the recovered signers.
    function publish(Attestation calldata a, bytes[] calldata signatures) external {
        uint256 threshold = _signatureThreshold;
        // Defensive: a valid threshold is always >= 1.
        if (threshold == 0) revert Errors.InvalidSignatureThreshold(0, _auditorCount);
        if (signatures.length < threshold) {
            revert Errors.InsufficientAuditorSignatures(signatures.length, threshold);
        }

        // EIP-712 digest over the attestation; every auditor signs this exact digest.
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

        // Recover and validate each signer: must be an authorised auditor and distinct.
        // Signers are tracked in a fixed-size scratch array to reject duplicates.
        address[] memory seen = new address[](signatures.length);
        bool leadSigned = false;
        for (uint256 i = 0; i < signatures.length; i++) {
            address recovered = ECDSA.recover(digest, signatures[i]);
            if (!hasRole(Roles.AUDITOR_ROLE, recovered)) revert Errors.UnknownAuditor(recovered);

            for (uint256 j = 0; j < i; j++) {
                if (seen[j] == recovered) revert Errors.DuplicateAuditorSignature(recovered);
            }
            seen[i] = recovered;

            if (recovered == a.auditor) leadSigned = true;
        }

        // The recorded lead auditor must be among the signers.
        if (!leadSigned) revert Errors.InvalidAuditorSignature();

        uint256 n = _attestations.length;

        // Monotonicity: timestamps and asOf must strictly increase.
        if (n > 0) {
            Attestation storage prev = _attestations[n - 1];
            if (a.timestamp <= prev.timestamp || a.asOf <= prev.asOf) {
                revert Errors.AttestationMonotonicityViolated(prev.timestamp, a.timestamp);
            }
            // Anti-batching: enforce a minimum wall-clock spacing so the per-step growth
            // cap cannot be compounded by publishing many attestations in one block.
            if (
                _minAttestationInterval > 0
                    && a.timestamp < prev.timestamp + _minAttestationInterval
            ) {
                revert Errors.AttestationTooSoon(prev.timestamp, a.timestamp, _minAttestationInterval);
            }
        }

        // Sanity: the publication timestamp must not claim a far-future time (±1h tolerance).
        if (a.timestamp > block.timestamp + 1 hours) {
            revert Errors.AttestationMonotonicityViolated(uint64(block.timestamp), a.timestamp);
        }

        // Sanity: the audit reference date (asOf) must not be in the future.
        if (a.asOf > block.timestamp) {
            revert Errors.AttestationInFuture(a.asOf, uint64(block.timestamp));
        }

        // Sanity: bound reserve growth between consecutive attestations. Skipped for the
        // first attestation (no baseline). new <= prev * (10_000 + maxGrowthBps) / 10_000.
        if (n > 0) {
            uint256 prevGrams = _attestations[n - 1].totalGrams;
            // Only an *increase* beyond the cap is rejected; decreases are always allowed.
            if (a.totalGrams > prevGrams) {
                uint256 maxAllowed = prevGrams + (prevGrams * _maxGrowthBps) / 10_000;
                if (a.totalGrams > maxAllowed) {
                    revert Errors.ReserveGrowthExceeded(prevGrams, a.totalGrams, _maxGrowthBps);
                }
            }
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
        // Guard: revoking an auditor must not leave the threshold unsatisfiable.
        if (_signatureThreshold > _auditorCount) {
            revert Errors.InvalidSignatureThreshold(_signatureThreshold, _auditorCount);
        }
    }

    // ──────────────────────────────────────────────────────────────────────
    // Threshold & growth configuration
    // ──────────────────────────────────────────────────────────────────────

    /// @notice Set the number of distinct auditor signatures required to publish.
    /// @dev DEFAULT_ADMIN / TREASURY only. Must be in [1, auditorCount].
    function setSignatureThreshold(uint256 newThreshold) external onlyRole(Roles.TREASURY_ROLE) {
        if (newThreshold == 0 || newThreshold > _auditorCount) {
            revert Errors.InvalidSignatureThreshold(newThreshold, _auditorCount);
        }
        _signatureThreshold = newThreshold;
        emit SignatureThresholdUpdated(newThreshold);
    }

    /// @notice Set the max allowed growth in totalGrams between consecutive attestations (bps).
    /// @dev Bounded by MAX_GROWTH_BPS_CAP so an extreme value cannot DoS publish via overflow.
    function setMaxGrowthBps(uint256 newMaxGrowthBps) external onlyRole(Roles.TREASURY_ROLE) {
        if (newMaxGrowthBps > MAX_GROWTH_BPS_CAP) {
            revert Errors.InvalidGrowthCap(newMaxGrowthBps, MAX_GROWTH_BPS_CAP);
        }
        _maxGrowthBps = newMaxGrowthBps;
        emit MaxGrowthBpsUpdated(newMaxGrowthBps);
    }

    /// @notice Set the minimum wall-clock spacing (seconds) between attestations.
    function setMinAttestationInterval(uint256 newInterval) external onlyRole(Roles.TREASURY_ROLE) {
        _minAttestationInterval = newInterval;
        emit MinAttestationIntervalUpdated(newInterval);
    }

    function signatureThreshold() external view returns (uint256) {
        return _signatureThreshold;
    }

    function auditorCount() external view returns (uint256) {
        return _auditorCount;
    }

    function maxGrowthBps() external view returns (uint256) {
        return _maxGrowthBps;
    }

    function minAttestationInterval() external view returns (uint256) {
        return _minAttestationInterval;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Role hooks — keep _auditorCount in sync
    // ──────────────────────────────────────────────────────────────────────

    function _grantRole(bytes32 role, address account) internal override returns (bool granted) {
        granted = super._grantRole(role, account);
        if (granted && role == Roles.AUDITOR_ROLE) {
            _auditorCount++;
        }
    }

    function _revokeRole(bytes32 role, address account) internal override returns (bool revoked) {
        revoked = super._revokeRole(role, account);
        if (revoked && role == Roles.AUDITOR_ROLE) {
            _auditorCount--;
        }
    }

    function DOMAIN_SEPARATOR() external view returns (bytes32) {
        return _domainSeparatorV4();
    }
}
