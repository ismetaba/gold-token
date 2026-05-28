// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { IBurnController } from "../src/interfaces/IBurnController.sol";
import { Roles } from "../src/libraries/Roles.sol";

contract BurnControllerTest is BaseTest {
    // ──────────────────────────────────────────────────────────────────────
    // Helpers
    // ──────────────────────────────────────────────────────────────────────

    /// @dev Build a compliance officer EIP-712 signature for operatorBurn.
    function _signOperatorBurn(
        address from,
        uint256 amount,
        bytes32 reasonHash,
        uint256 nonce,
        uint256 deadline
    ) internal view returns (bytes memory) {
        bytes32 OPERATOR_BURN_TYPEHASH = keccak256(
            "OperatorBurn(address from,uint256 amount,bytes32 reasonHash,uint256 nonce,uint256 deadline)"
        );
        bytes32 structHash = keccak256(
            abi.encode(OPERATOR_BURN_TYPEHASH, from, amount, reasonHash, nonce, deadline)
        );
        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", burner.DOMAIN_SEPARATOR(), structHash)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(complianceOfficerPk, digest);
        return abi.encodePacked(r, s, v);
    }

    /// @dev Mint `gross` tokens via MintController and approve `netAmount` to the burner.
    ///      Returns the net amount alice actually received (gross minus 0.25% mint fee).
    function _mintTokens(address to, uint256 gross) internal returns (uint256 netAmount) {
        _setKyc(to, TR);
        _publishReserve(gross * 2);
        bytes32 pid = _proposeAndApproveMint(to, gross, TR, keccak256(abi.encode(to, gross)));
        vm.prank(executor);
        minter.executeMint(pid);
        netAmount = _netMintAmount(gross);
        vm.prank(to);
        token.approve(address(burner), netAmount);
    }

    /// @dev Recompute the reqId the same way BurnController does.
    function _reqId(IBurnController.RedemptionRequest memory req)
        internal
        view
        returns (bytes32)
    {
        return keccak256(
            abi.encode(
                req.from,
                req.amount,
                req.redemptionType,
                keccak256(bytes(req.deliveryRef)),
                req.offChainOrderId,
                block.chainid,
                address(burner)
            )
        );
    }

    /// @dev Freeze a wallet (compliance action) so it becomes clawback-eligible.
    function _freeze(address wallet) internal {
        vm.prank(complianceOfficer);
        compliance.freeze(wallet, keccak256("compliance-action"));
    }

    // ──────────────────────────────────────────────────────────────────────
    // Burn fee tests
    // ──────────────────────────────────────────────────────────────────────

    function test_burnFee_constant() public view {
        assertEq(burner.BURN_FEE_BPS(), 25, "BURN_FEE_BPS == 25 bps == 0.25%");
    }

    function test_burnFee_emittedOnRedemption() public {
        uint256 net = _mintTokens(alice, 200 * 1e18);
        uint256 burnAmount = net; // burn everything alice has

        IBurnController.RedemptionRequest memory req = IBurnController.RedemptionRequest({
            from: alice,
            amount: burnAmount,
            redemptionType: IBurnController.RedemptionType.CASH_BACK,
            offChainOrderId: keccak256("order-fee-test"),
            deliveryRef: "IBAN123"
        });

        uint256 expectedFee = burnAmount * 25 / 10_000;

        vm.prank(burnOperator);
        vm.expectEmit(true, true, false, true);
        bytes32 reqId = _reqId(req);
        emit IBurnController.BurnFeeCollected(reqId, alice, expectedFee);
        burner.requestRedemption(req);

        // All tokens burned (full amount burned on-chain; off-chain delivers amount - fee)
        assertEq(token.balanceOf(alice), 0, "alice balance is zero after burn");
    }

    // ──────────────────────────────────────────────────────────────────────
    // Minimum redemption tests
    // ──────────────────────────────────────────────────────────────────────

    function test_minPhysical_100grams() public view {
        assertEq(burner.minPhysicalGrams(), 100 * 1e18, "minimum physical is 100 grams");
    }

    function test_minPhysical_belowMinimumReverts() public {
        uint256 net = _mintTokens(alice, 200 * 1e18);
        uint256 burnAmount = 50 * 1e18; // below 100g minimum
        require(burnAmount <= net, "test setup: alice has enough tokens");

        vm.prank(alice);
        token.approve(address(burner), burnAmount);

        IBurnController.RedemptionRequest memory req = IBurnController.RedemptionRequest({
            from: alice,
            amount: burnAmount,
            redemptionType: IBurnController.RedemptionType.PHYSICAL,
            offChainOrderId: keccak256("order-below-min"),
            deliveryRef: ""
        });

        vm.prank(burnOperator);
        vm.expectRevert(
            abi.encodeWithSelector(
                Errors.BelowMinimumRedemption.selector, burnAmount, 100 * 1e18
            )
        );
        burner.requestRedemption(req);
    }

    function test_minPhysical_exactMinimumSucceeds() public {
        uint256 net = _mintTokens(alice, 200 * 1e18);
        uint256 burnAmount = 100 * 1e18; // exactly the minimum
        require(burnAmount <= net, "test setup: alice has enough tokens");

        vm.prank(alice);
        token.approve(address(burner), burnAmount);

        IBurnController.RedemptionRequest memory req = IBurnController.RedemptionRequest({
            from: alice,
            amount: burnAmount,
            redemptionType: IBurnController.RedemptionType.PHYSICAL,
            offChainOrderId: keccak256("order-exact-min"),
            deliveryRef: "vault-addr"
        });

        vm.prank(burnOperator);
        burner.requestRedemption(req); // should succeed
    }

    // ──────────────────────────────────────────────────────────────────────
    // Deadline tests
    // ──────────────────────────────────────────────────────────────────────

    function test_operatorBurn_expiredDeadlineReverts() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);

        uint256 deadline = block.timestamp - 1; // past
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        vm.expectRevert(abi.encodeWithSelector(Errors.DeadlineExpired.selector, deadline));
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);
    }

    function test_operatorBurn_validDeadlineSucceeds() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);
        assertEq(token.balanceOf(alice), net);

        // Clawback eligibility: target must be under an active compliance action.
        _freeze(alice);

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);

        assertEq(token.balanceOf(alice), 0);
    }

    // ──────────────────────────────────────────────────────────────────────
    // operatorBurn: clawback compliance gate + pause semantics
    // ──────────────────────────────────────────────────────────────────────

    function test_operatorBurn_revertsIfTargetNotFrozenOrSanctioned() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);

        // alice is in good standing — clawback must be rejected even with a valid signature.
        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        vm.expectRevert(Errors.NotAuthorized.selector);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);
    }

    function test_operatorBurn_worksWhenSanctioned() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);

        vm.prank(complianceOfficer);
        compliance.setSanctioned(alice, true);

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);

        assertEq(token.balanceOf(alice), 0, "sanctioned wallet clawed back");
    }

    function test_operatorBurn_worksWhilePaused() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);
        _freeze(alice);

        // Pause the token: normal transfers/burns are blocked, but the emergency
        // compliance clawback must still execute.
        vm.prank(pauser);
        token.pause();
        assertTrue(token.paused());

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);

        assertEq(token.balanceOf(alice), 0, "clawback executes while paused");
    }

    function test_operatorBurn_noAllowanceRequired() public {
        // _mintTokens approves the burner; revoke that approval to prove operatorBurn
        // does not rely on an allowance from the (non-cooperative) target.
        uint256 net = _mintTokens(alice, 50 * 1e18);
        vm.prank(alice);
        token.approve(address(burner), 0);
        _freeze(alice);

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);
        assertEq(token.balanceOf(alice), 0);
    }

    // ──────────────────────────────────────────────────────────────────────
    // reqId binding: redemptionType + deliveryRef
    // ──────────────────────────────────────────────────────────────────────

    function test_requestRedemption_redemptionTypeProducesDistinctReqIds() public {
        // Mint enough for two physical-min redemptions.
        uint256 net = _mintTokens(alice, 400 * 1e18);
        vm.prank(alice);
        token.approve(address(burner), net);

        bytes32 orderId = keccak256("same-order");
        IBurnController.RedemptionRequest memory reqCash = IBurnController.RedemptionRequest({
            from: alice,
            amount: 100 * 1e18,
            redemptionType: IBurnController.RedemptionType.CASH_BACK,
            offChainOrderId: orderId,
            deliveryRef: "ref"
        });
        IBurnController.RedemptionRequest memory reqPhys = IBurnController.RedemptionRequest({
            from: alice,
            amount: 100 * 1e18,
            redemptionType: IBurnController.RedemptionType.PHYSICAL,
            offChainOrderId: orderId,
            deliveryRef: "ref"
        });

        vm.prank(burnOperator);
        bytes32 id1 = burner.requestRedemption(reqCash);
        vm.prank(burnOperator);
        bytes32 id2 = burner.requestRedemption(reqPhys);

        assertTrue(id1 != id2, "differing redemptionType yields different reqId");
        assertEq(id1, _reqId(reqCash));
        assertEq(id2, _reqId(reqPhys));
        // Both executed successfully.
        (, bool e1,) = burner.getRedemption(id1);
        (, bool e2,) = burner.getRedemption(id2);
        assertTrue(e1 && e2, "both redemptions executed");
    }

    function test_requestRedemption_deliveryRefProducesDistinctReqIds() public {
        uint256 net = _mintTokens(alice, 400 * 1e18);
        vm.prank(alice);
        token.approve(address(burner), net);

        bytes32 orderId = keccak256("order-x");
        IBurnController.RedemptionRequest memory reqA = IBurnController.RedemptionRequest({
            from: alice,
            amount: 100 * 1e18,
            redemptionType: IBurnController.RedemptionType.CASH_BACK,
            offChainOrderId: orderId,
            deliveryRef: "IBAN-A"
        });
        IBurnController.RedemptionRequest memory reqB = IBurnController.RedemptionRequest({
            from: alice,
            amount: 100 * 1e18,
            redemptionType: IBurnController.RedemptionType.CASH_BACK,
            offChainOrderId: orderId,
            deliveryRef: "IBAN-B"
        });

        vm.prank(burnOperator);
        bytes32 id1 = burner.requestRedemption(reqA);
        vm.prank(burnOperator);
        bytes32 id2 = burner.requestRedemption(reqB);

        assertTrue(id1 != id2, "differing deliveryRef yields different reqId");
    }

    function test_operatorBurn_invalidSignerReverts() public {
        uint256 net = _mintTokens(alice, 50 * 1e18);

        uint256 deadline = block.timestamp + 1 hours;
        // Sign with the auditor key (not the compliance officer)
        bytes32 TYPEHASH = keccak256(
            "OperatorBurn(address from,uint256 amount,bytes32 reasonHash,uint256 nonce,uint256 deadline)"
        );
        bytes32 structHash = keccak256(
            abi.encode(TYPEHASH, alice, net, keccak256("reason"), uint256(0), deadline)
        );
        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", burner.DOMAIN_SEPARATOR(), structHash)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(auditorPk, digest); // auditor is not authorised
        bytes memory badSig = abi.encodePacked(r, s, v);

        vm.prank(burnOperator);
        vm.expectRevert(Errors.NotAuthorized.selector);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, badSig);
    }
}
