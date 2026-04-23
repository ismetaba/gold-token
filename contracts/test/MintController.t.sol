// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";

contract MintControllerTest is BaseTest {
    function test_reserveInvariant_mintBlockedIfExceedsAttested() public {
        _setKyc(alice, TR);
        _publishReserve(500 * 1e18); // 500 gram kasada

        // 600 gram mint girişimi → invaryant ihlali
        bytes32 pid =
            _proposeAndApproveMint(alice, 600 * 1e18, TR, keccak256("alloc-over"));

        vm.prank(executor);
        vm.expectRevert(
            abi.encodeWithSelector(
                Errors.ReserveInvariantViolated.selector, 600 * 1e18, 500 * 1e18
            )
        );
        minter.executeMint(pid);
    }

    function test_reserveStaleness_blocksMintAfterMaxAge() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32 pid =
            _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-stale"));

        // 36 gün ileri sar (maxAge = 35)
        vm.warp(block.timestamp + 36 days);

        vm.prank(executor);
        vm.expectRevert(); // StaleReserveAttestation
        minter.executeMint(pid);
    }

    function test_multisig_insufficientApprovalsBlocksExecute() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-xy");
        vm.prank(proposer);
        bytes32 pid = minter.proposeMint(
            _mintRequest(alice, 50 * 1e18, keccak256("alloc-few"), bars, TR)
        );

        // Sadece 2 onay (eşik 3)
        vm.prank(approvers[0]);
        minter.approveMint(pid);
        vm.prank(approvers[1]);
        minter.approveMint(pid);

        vm.prank(executor);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.InsufficientApprovals.selector, 2, 3)
        );
        minter.executeMint(pid);
    }

    function test_multisig_sameApproverCannotDoubleApprove() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-zz");
        vm.prank(proposer);
        bytes32 pid = minter.proposeMint(
            _mintRequest(alice, 50 * 1e18, keccak256("alloc-dup"), bars, TR)
        );

        vm.prank(approvers[0]);
        minter.approveMint(pid);

        vm.prank(approvers[0]);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.ProposalAlreadyApprovedBy.selector, approvers[0])
        );
        minter.approveMint(pid);
    }

    function test_allocationId_cannotBeReused() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32 allocId = keccak256("alloc-reuse");
        bytes32 pid = _proposeAndApproveMint(alice, 100 * 1e18, TR, allocId);
        vm.prank(executor);
        minter.executeMint(pid);

        // Aynı allocationId ile tekrar propose → fail
        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-reuse");
        vm.prank(proposer);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.AllocationAlreadyUsed.selector, allocId)
        );
        minter.proposeMint(_mintRequest(alice, 50 * 1e18, allocId, bars, TR));
    }

    function test_cancel_byProposer() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-cancel");
        vm.prank(proposer);
        bytes32 pid = minter.proposeMint(
            _mintRequest(alice, 50 * 1e18, keccak256("alloc-cancel"), bars, TR)
        );

        vm.prank(proposer);
        minter.cancelMint(pid, keccak256("ops-error"));

        vm.prank(executor);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.ProposalNotFound.selector, pid)
        );
        minter.executeMint(pid);
    }
}
