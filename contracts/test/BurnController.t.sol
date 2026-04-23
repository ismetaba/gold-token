// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { IBurnController } from "../src/interfaces/IBurnController.sol";
import { Roles } from "../src/libraries/Roles.sol";

contract BurnControllerTest is BaseTest {
    // ──────────────────────────────────────────────────────────────────────
    // Yardımcılar
    // ──────────────────────────────────────────────────────────────────────

    /// @dev Compliance officer'ın EIP-712 operatorBurn imzasını oluşturur.
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

    /// @dev Alice'e token mintler (operatorBurn testleri için bakiye gerekli).
    function _mintTokens(address to, uint256 amount) internal {
        _setKyc(to, TR);
        _publishReserve(amount * 2);
        bytes32 pid = _proposeAndApproveMint(to, amount, TR, keccak256(abi.encode(to, amount)));
        vm.prank(executor);
        minter.executeMint(pid);
        // BurnController'ın pull-burn yapabilmesi için approve
        vm.prank(to);
        token.approve(address(burner), amount);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Deadline testleri
    // ──────────────────────────────────────────────────────────────────────

    function test_operatorBurn_expiredDeadlineReverts() public {
        _mintTokens(alice, 50 * 1e18);

        uint256 deadline = block.timestamp - 1; // geçmiş
        bytes memory sig = _signOperatorBurn(alice, 50 * 1e18, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        vm.expectRevert(abi.encodeWithSelector(Errors.DeadlineExpired.selector, deadline));
        burner.operatorBurn(alice, 50 * 1e18, keccak256("reason"), deadline, sig);
    }

    function test_operatorBurn_validDeadlineSucceeds() public {
        _mintTokens(alice, 50 * 1e18);
        assertEq(token.balanceOf(alice), 50 * 1e18);

        uint256 deadline = block.timestamp + 1 hours;
        bytes memory sig = _signOperatorBurn(alice, 50 * 1e18, keccak256("reason"), 0, deadline);

        vm.prank(burnOperator);
        burner.operatorBurn(alice, 50 * 1e18, keccak256("reason"), deadline, sig);

        assertEq(token.balanceOf(alice), 0);
    }

    function test_operatorBurn_invalidSignerReverts() public {
        _mintTokens(alice, 50 * 1e18);

        uint256 deadline = block.timestamp + 1 hours;
        // Yanlış key ile imzala (auditor, compliance officer değil)
        bytes32 TYPEHASH = keccak256(
            "OperatorBurn(address from,uint256 amount,bytes32 reasonHash,uint256 nonce,uint256 deadline)"
        );
        bytes32 structHash = keccak256(
            abi.encode(TYPEHASH, alice, 50 * 1e18, keccak256("reason"), uint256(0), deadline)
        );
        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", burner.DOMAIN_SEPARATOR(), structHash)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(auditorPk, digest); // auditor, yetkisiz
        bytes memory badSig = abi.encodePacked(r, s, v);

        vm.prank(burnOperator);
        vm.expectRevert(Errors.NotAuthorized.selector);
        burner.operatorBurn(alice, 50 * 1e18, keccak256("reason"), deadline, badSig);
    }
}
