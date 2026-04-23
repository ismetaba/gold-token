// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { Roles } from "../src/libraries/Roles.sol";
import { IMintController } from "../src/interfaces/IMintController.sol";

contract MintControllerTest is BaseTest {
    function test_reserveInvariant_mintBlockedIfExceedsAttested() public {
        _setKyc(alice, TR);
        _publishReserve(500 * 1e18); // 500 grams in vault

        // Attempt to mint 600 grams — reserve invariant violation
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

        // Advance 36 days (maxAge = 35)
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

        // Only 2 approvals (threshold is 3)
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

        // Same allocationId on a second propose must revert
        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-reuse");
        vm.prank(proposer);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.AllocationAlreadyUsed.selector, allocId)
        );
        minter.proposeMint(_mintRequest(alice, 50 * 1e18, allocId, bars, TR));
    }

    // ──────────────────────────────────────────────────────────────────────
    // Mint fee tests
    // ──────────────────────────────────────────────────────────────────────

    function test_mintFee_recipientReceivesNetAmount() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        uint256 gross = 100 * 1e18;
        bytes32 pid = _proposeAndApproveMint(alice, gross, TR, keccak256("alloc-fee-1"));
        vm.prank(executor);
        minter.executeMint(pid);

        // Alice receives gross - 0.25% fee
        uint256 expectedNet = _netMintAmount(gross); // 99.75e18
        assertEq(token.balanceOf(alice), expectedNet, "recipient net amount");

        // Treasury receives the fee
        uint256 expectedFee = gross - expectedNet; // 0.25e18
        assertEq(token.balanceOf(treasury), expectedFee, "treasury fee amount");
    }

    function test_mintFee_totalSupplyEqualsGrossAmount() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        uint256 gross = 200 * 1e18;
        bytes32 pid = _proposeAndApproveMint(alice, gross, TR, keccak256("alloc-supply"));
        vm.prank(executor);
        minter.executeMint(pid);

        // Total supply == gross (fee tokens are still in circulation as treasury balance)
        assertEq(token.totalSupply(), gross, "total supply equals gross");
    }

    function test_mintFee_feeBpsConstant() public view {
        assertEq(minter.MINT_FEE_BPS(), 25, "MINT_FEE_BPS == 25 bps == 0.25%");
    }

    function test_mintFee_feeRecipientIsSetToTreasury() public view {
        assertEq(minter.feeRecipient(), treasury, "feeRecipient is treasury");
    }

    // ──────────────────────────────────────────────────────────────────────
    // Rate limit tests
    // ──────────────────────────────────────────────────────────────────────

    function test_rateLimit_blocksExcessiveMint() public {
        _setKyc(alice, TR);
        _publishReserve(10_000 * 1e18);

        // 100 gram/day rate limit
        vm.prank(treasury);
        minter.setRateLimit(1 days, 100 * 1e18);

        // Exactly fills the limit — succeeds
        bytes32 pid1 = _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-rl-1"));
        vm.prank(executor);
        minter.executeMint(pid1);

        // 1 gram more — exceeds limit
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

        // Advance past the window — counter resets
        vm.warp(block.timestamp + 1 days);

        bytes32 pid2 = _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-rw-2"));
        vm.prank(executor);
        minter.executeMint(pid2); // succeeds — new window

        // Alice receives 2 × net amounts (fee goes to treasury each time)
        assertEq(token.balanceOf(alice), 2 * _netMintAmount(100 * 1e18));
    }

    function test_rateLimit_disabledWhenZero() public {
        _setKyc(alice, TR);
        _publishReserve(10_000 * 1e18);

        // Default: rate limit disabled (0)
        (uint256 window, uint256 max) = minter.rateLimit();
        assertEq(window, 0);
        assertEq(max, 0);

        // Large mint succeeds
        uint256 gross = 5_000 * 1e18;
        bytes32 pid = _proposeAndApproveMint(alice, gross, TR, keccak256("alloc-nrl"));
        vm.prank(executor);
        minter.executeMint(pid);

        assertEq(token.balanceOf(alice), _netMintAmount(gross));
    }

    function test_reProposalAfterCancel_clearsStaleApprovals() public {
        _setKyc(alice, TR);
        _setKyc(bob, TR);
        _publishReserve(10_000 * 1e18);

        bytes32 allocId = keccak256("alloc-stale-approval");

        // Step 1: Propose a mint for alice
        bytes32[] memory bars1 = new bytes32[](1);
        bars1[0] = keccak256("bar-v1");
        vm.prank(proposer);
        bytes32 pid = minter.proposeMint(
            _mintRequest(alice, 100 * 1e18, allocId, bars1, TR)
        );

        // Step 2: Get 2 of 3 required approvals
        vm.prank(approvers[0]);
        minter.approveMint(pid);
        vm.prank(approvers[1]);
        minter.approveMint(pid);

        // Step 3: Cancel the proposal
        vm.prank(proposer);
        minter.cancelMint(pid, keccak256("changed-params"));

        // Step 4: Re-propose with DIFFERENT params (bob, different amount)
        bytes32[] memory bars2 = new bytes32[](1);
        bars2[0] = keccak256("bar-v2");
        vm.prank(proposer);
        bytes32 pid2 = minter.proposeMint(
            _mintRequest(bob, 200 * 1e18, allocId, bars2, TR)
        );
        // proposalId is deterministic from allocationId
        assertEq(pid2, pid, "proposalId unchanged");

        // Step 5: Verify old approvals are cleared — proposal has 0 approvers
        IMintController.Proposal memory p = minter.getProposal(pid2);
        assertEq(p.approvers.length, 0, "stale approvers must be cleared");
        assertEq(p.req.to, bob, "new recipient");
        assertEq(p.req.amount, 200 * 1e18, "new amount");

        // Step 6: Attempting execution fails — no approvals yet
        vm.prank(executor);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.InsufficientApprovals.selector, 0, 3)
        );
        minter.executeMint(pid2);

        // Step 7: Full new approval round required — previous approvers can approve again
        vm.prank(approvers[0]);
        minter.approveMint(pid2);
        vm.prank(approvers[1]);
        minter.approveMint(pid2);
        vm.prank(approvers[2]);
        minter.approveMint(pid2);

        // Step 8: Now execution succeeds with the new params
        vm.prank(executor);
        minter.executeMint(pid2);

        // Bob receives net tokens (not alice)
        assertEq(token.balanceOf(bob), _netMintAmount(200 * 1e18), "bob received tokens");
        assertEq(token.balanceOf(alice), 0, "alice received nothing");
    }

    // ──────────────────────────────────────────────────────────────────────
    // totalApprovers sync tests (MEDIUM-2)
    // ──────────────────────────────────────────────────────────────────────

    function test_totalApprovers_syncOnGrant() public {
        // Initial state: 5 approvers from setUp
        assertEq(minter.totalApprovers(), 5, "initial totalApprovers");

        // Grant a new approver
        address newApprover = makeAddr("newApprover");
        vm.prank(treasury);
        minter.grantRole(Roles.MINT_APPROVER_ROLE, newApprover);

        assertEq(minter.totalApprovers(), 6, "totalApprovers after grant");
    }

    function test_totalApprovers_syncOnRevoke() public {
        assertEq(minter.totalApprovers(), 5, "initial totalApprovers");

        vm.prank(treasury);
        minter.revokeRole(Roles.MINT_APPROVER_ROLE, approvers[4]);

        assertEq(minter.totalApprovers(), 4, "totalApprovers after revoke");
    }

    function test_totalApprovers_thresholdValidationUsesUpdatedCount() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        // Grant a 6th approver
        address newApprover = makeAddr("newApprover");
        vm.prank(treasury);
        minter.grantRole(Roles.MINT_APPROVER_ROLE, newApprover);
        assertEq(minter.totalApprovers(), 6, "totalApprovers is 6");

        // Setting threshold to 6 should work now (was max 5 before the fix)
        vm.prank(treasury);
        minter.setApprovalThreshold(6);
        assertEq(minter.approvalThreshold(), 6);

        // Attempting threshold of 7 should fail
        vm.prank(treasury);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.InsufficientApprovals.selector, 0, 7)
        );
        minter.setApprovalThreshold(7);
    }

    function test_totalApprovers_doubleGrantNoDoubleCount() public {
        assertEq(minter.totalApprovers(), 5, "initial totalApprovers");

        // Granting to an existing approver should NOT increment
        vm.prank(treasury);
        minter.grantRole(Roles.MINT_APPROVER_ROLE, approvers[0]);

        assertEq(minter.totalApprovers(), 5, "totalApprovers unchanged on double grant");
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
