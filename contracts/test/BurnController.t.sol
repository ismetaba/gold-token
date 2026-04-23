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
        bytes32 reqId = keccak256(
            abi.encode(alice, burnAmount, req.offChainOrderId, block.chainid, address(burner))
        );
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

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, net, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, net, keccak256("reason"), deadline, sig);

        assertEq(token.balanceOf(alice), 0);
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
