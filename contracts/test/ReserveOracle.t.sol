// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { ReserveOracle } from "../src/ReserveOracle.sol";
import { IReserveOracle } from "../src/interfaces/IReserveOracle.sol";
import { Errors } from "../src/libraries/Errors.sol";

contract ReserveOracleTest is BaseTest {
    function test_publish_incrementsCount() public {
        assertEq(oracle.attestationCount(), 0);
        _publishReserve(1000 * 1e18);
        assertEq(oracle.attestationCount(), 1);
        assertEq(oracle.latestAttestedGrams(), 1000 * 1e18);
    }

    function test_publish_monotonicityEnforced() public {
        _publishReserve(1000 * 1e18);

        // Aynı timestamp ile ikinci yayın → fail
        IReserveOracle.Attestation memory a = IReserveOracle.Attestation({
            timestamp: uint64(block.timestamp),
            asOf: uint64(block.timestamp),
            totalGrams: 1100 * 1e18,
            merkleRoot: bytes32(uint256(0xAA)),
            ipfsCid: bytes32(uint256(0xBB)),
            auditor: auditor
        });

        bytes32 structHash = keccak256(
            abi.encode(
                keccak256(
                    "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
                ),
                a.timestamp, a.asOf, a.totalGrams, a.merkleRoot, a.ipfsCid, a.auditor
            )
        );
        bytes32 domain = oracle.DOMAIN_SEPARATOR();
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", domain, structHash));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(auditorPk, digest);

        vm.expectRevert();
        oracle.publish(a, abi.encodePacked(r, s, v));
    }

    function test_publish_unauthorizedAuditorRejected() public {
        uint256 fakePk = 0xBAD;
        address fakeAuditor = vm.addr(fakePk);

        IReserveOracle.Attestation memory a = IReserveOracle.Attestation({
            timestamp: uint64(block.timestamp),
            asOf: uint64(block.timestamp),
            totalGrams: 500 * 1e18,
            merkleRoot: bytes32(0),
            ipfsCid: bytes32(0),
            auditor: fakeAuditor
        });

        bytes32 structHash = keccak256(
            abi.encode(
                keccak256(
                    "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
                ),
                a.timestamp, a.asOf, a.totalGrams, a.merkleRoot, a.ipfsCid, a.auditor
            )
        );
        bytes32 domain = oracle.DOMAIN_SEPARATOR();
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", domain, structHash));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(fakePk, digest);

        vm.expectRevert(
            abi.encodeWithSelector(Errors.UnknownAuditor.selector, fakeAuditor)
        );
        oracle.publish(a, abi.encodePacked(r, s, v));
    }

    function test_timeSinceLatest_staleDetection() public {
        _publishReserve(1000 * 1e18);
        assertEq(oracle.timeSinceLatest(), 0);

        vm.warp(block.timestamp + 10 days);
        assertEq(oracle.timeSinceLatest(), 10 days);
    }
}
