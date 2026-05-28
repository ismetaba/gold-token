// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { ReserveOracle } from "../src/ReserveOracle.sol";
import { IReserveOracle } from "../src/interfaces/IReserveOracle.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { Roles } from "../src/libraries/Roles.sol";
import { IAccessControl } from "@openzeppelin/contracts/access/IAccessControl.sol";

contract ReserveOracleTest is BaseTest {
    function test_publish_incrementsCount() public {
        assertEq(oracle.attestationCount(), 0);
        _publishReserve(1000 * 1e18);
        assertEq(oracle.attestationCount(), 1);
        assertEq(oracle.latestAttestedGrams(), 1000 * 1e18);
    }

    bytes32 constant ATTESTATION_TYPEHASH = keccak256(
        "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
    );

    /// @dev Build the EIP-712 digest for an attestation.
    function _digest(IReserveOracle.Attestation memory a) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                ATTESTATION_TYPEHASH,
                a.timestamp, a.asOf, a.totalGrams, a.merkleRoot, a.ipfsCid, a.auditor
            )
        );
        return keccak256(abi.encodePacked("\x19\x01", oracle.DOMAIN_SEPARATOR(), structHash));
    }

    function _sign(uint256 pk, bytes32 digest) internal pure returns (bytes memory) {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pk, digest);
        return abi.encodePacked(r, s, v);
    }

    function _att(uint256 totalGrams) internal view returns (IReserveOracle.Attestation memory) {
        return IReserveOracle.Attestation({
            timestamp: uint64(block.timestamp),
            asOf: uint64(block.timestamp),
            totalGrams: totalGrams,
            merkleRoot: bytes32(uint256(0xAA)),
            ipfsCid: bytes32(uint256(0xBB)),
            auditor: auditor
        });
    }

    function test_publish_monotonicityEnforced() public {
        _publishReserve(1000 * 1e18);

        // Same timestamp as previous → monotonicity violation
        IReserveOracle.Attestation memory a = _att(1100 * 1e18);
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditorPk, digest);
        sigs[1] = _sign(auditor2Pk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(
                Errors.AttestationMonotonicityViolated.selector,
                uint64(block.timestamp), uint64(block.timestamp)
            )
        );
        oracle.publish(a, sigs);
    }

    function test_publish_unauthorizedAuditorRejected() public {
        uint256 fakePk = 0xBAD;
        address fakeAuditor = vm.addr(fakePk);

        IReserveOracle.Attestation memory a = _att(500 * 1e18);
        a.auditor = fakeAuditor;
        bytes32 digest = _digest(a);

        // Threshold is 2: one good signer + one unknown signer.
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditorPk, digest);
        sigs[1] = _sign(fakePk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(Errors.UnknownAuditor.selector, fakeAuditor)
        );
        oracle.publish(a, sigs);
    }

    // ──────────────────────────────────────────────────────────────────────
    // N-of-M signature threshold
    // ──────────────────────────────────────────────────────────────────────

    function test_defaultThresholdIsTwo() public view {
        assertEq(oracle.signatureThreshold(), 2);
        assertEq(oracle.auditorCount(), 2);
    }

    function test_publish_belowThresholdReverts() public {
        IReserveOracle.Attestation memory a = _att(1000 * 1e18);
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](1);
        sigs[0] = _sign(auditorPk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(Errors.InsufficientAuditorSignatures.selector, 1, 2)
        );
        oracle.publish(a, sigs);
    }

    function test_publish_duplicateSignerRejected() public {
        IReserveOracle.Attestation memory a = _att(1000 * 1e18);
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditorPk, digest);
        sigs[1] = _sign(auditorPk, digest); // duplicate

        vm.expectRevert(
            abi.encodeWithSelector(Errors.DuplicateAuditorSignature.selector, auditor)
        );
        oracle.publish(a, sigs);
    }

    function test_publish_twoDistinctAuditorsSucceeds() public {
        _publishReserve(1000 * 1e18);
        assertEq(oracle.attestationCount(), 1);
        assertEq(oracle.latestAttestedGrams(), 1000 * 1e18);
    }

    function test_publish_leadAuditorMustSign() public {
        // Threshold 2 with two valid signers, but neither is `a.auditor`.
        // Add a third auditor and lower nothing; sign with auditor2 + auditor3, set a.auditor=auditor (not a signer).
        address auditor3 = makeAddr("auditor3");
        // auditor3 has no private key we control; use a fresh pk instead.
        uint256 auditor3Pk = 0xA3;
        auditor3 = vm.addr(auditor3Pk);
        vm.prank(treasury);
        oracle.authorizeAuditor(auditor3);

        IReserveOracle.Attestation memory a = _att(1000 * 1e18);
        a.auditor = auditor; // lead = auditor, but auditor does NOT sign
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditor2Pk, digest);
        sigs[1] = _sign(auditor3Pk, digest);

        vm.expectRevert(Errors.InvalidAuditorSignature.selector);
        oracle.publish(a, sigs);
    }

    function test_setSignatureThreshold_onlyTreasury() public {
        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector,
                address(this),
                Roles.TREASURY_ROLE
            )
        );
        oracle.setSignatureThreshold(1);

        vm.prank(treasury);
        oracle.setSignatureThreshold(1);
        assertEq(oracle.signatureThreshold(), 1);
    }

    function test_setSignatureThreshold_exceedsAuditorsReverts() public {
        vm.prank(treasury);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.InvalidSignatureThreshold.selector, 3, 2)
        );
        oracle.setSignatureThreshold(3);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Sanity bounds
    // ──────────────────────────────────────────────────────────────────────

    function test_publish_asOfInFutureReverts() public {
        IReserveOracle.Attestation memory a = _att(1000 * 1e18);
        a.asOf = uint64(block.timestamp + 1 days);
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditorPk, digest);
        sigs[1] = _sign(auditor2Pk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(
                Errors.AttestationInFuture.selector, a.asOf, uint64(block.timestamp)
            )
        );
        oracle.publish(a, sigs);
    }

    function test_publish_growthCapEnforced() public {
        _publishReserve(1000 * 1e18); // baseline
        assertEq(oracle.maxGrowthBps(), 5000); // +50% default

        // Jump to >50% growth: 1600 grams (>1500 cap)
        vm.warp(block.timestamp + 1);
        IReserveOracle.Attestation memory a = _att(1600 * 1e18);
        bytes32 digest = _digest(a);
        bytes[] memory sigs = new bytes[](2);
        sigs[0] = _sign(auditorPk, digest);
        sigs[1] = _sign(auditor2Pk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(
                Errors.ReserveGrowthExceeded.selector, 1000 * 1e18, 1600 * 1e18, 5000
            )
        );
        oracle.publish(a, sigs);
    }

    function test_publish_growthWithinCapSucceeds() public {
        _publishReserve(1000 * 1e18);
        vm.warp(block.timestamp + 1);
        _publishReserve(1500 * 1e18); // exactly +50% — allowed
        assertEq(oracle.latestAttestedGrams(), 1500 * 1e18);
    }

    function test_publish_firstAttestationSkipsGrowthCap() public {
        // A large first attestation must succeed (no baseline to compare against).
        _publishReserve(1_000_000 * 1e18);
        assertEq(oracle.latestAttestedGrams(), 1_000_000 * 1e18);
    }

    function test_timeSinceLatest_staleDetection() public {
        _publishReserve(1000 * 1e18);
        assertEq(oracle.timeSinceLatest(), 0);

        vm.warp(block.timestamp + 10 days);
        assertEq(oracle.timeSinceLatest(), 10 days);
    }
}
