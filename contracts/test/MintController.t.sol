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

    // ──────────────────────────────────────────────────────────────────────
    // Hız sınırı testleri
    // ──────────────────────────────────────────────────────────────────────

    function test_rateLimit_blocksExcessiveMint() public {
        _setKyc(alice, TR);
        _publishReserve(10_000 * 1e18);

        // 100 gram/gün sınırı
        vm.prank(treasury);
        minter.setRateLimit(1 days, 100 * 1e18);

        // 100 gram: limit tam dolduruyor — başarılı
        bytes32 pid1 = _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-rl-1"));
        vm.prank(executor);
        minter.executeMint(pid1);

        // 1 gram daha: limit aşıyor → fail
        bytes32 pid2 = _proposeAndApproveMint(alice, 1 * 1e18, TR, keccak256("alloc-rl-2"));
        vm.prank(executor);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.RateLimitExceeded.selector, 101 * 1e18, 100 * 1e18)
        );
        minter.executeMint(pid2);
    }

    function test_rateLimit_resetsAfterWindow() public {
        _setKyc(alice, TR);
        _publishReserve(10_000 * 1e18);

        vm.prank(treasury);
        minter.setRateLimit(1 days, 100 * 1e18);

        bytes32 pid1 = _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-rw-1"));
        vm.prank(executor);
        minter.executeMint(pid1);

        // Pencere geçtikten sonra sıfırlanmalı
        vm.warp(block.timestamp + 1 days);

        bytes32 pid2 = _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-rw-2"));
        vm.prank(executor);
        minter.executeMint(pid2); // başarılı — yeni pencere

        assertEq(token.balanceOf(alice), 200 * 1e18);
    }

    function test_rateLimit_disabledWhenZero() public {
        _setKyc(alice, TR);
        _publishReserve(10_000 * 1e18);

        // Varsayılan: hız sınırı yok (0)
        (uint256 window, uint256 max) = minter.rateLimit();
        assertEq(window, 0);
        assertEq(max, 0);

        // Büyük miktarda mint başarılı
        bytes32 pid = _proposeAndApproveMint(alice, 5_000 * 1e18, TR, keccak256("alloc-nrl"));
        vm.prank(executor);
        minter.executeMint(pid);

        assertEq(token.balanceOf(alice), 5_000 * 1e18);
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
