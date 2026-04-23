// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";

contract GoldTokenTest is BaseTest {
    function test_initialState() public view {
        assertEq(token.name(), "GOLD Gold");
        assertEq(token.symbol(), "GOLD");
        assertEq(token.decimals(), 18);
        assertEq(token.totalSupply(), 0);
        assertEq(token.complianceRegistry(), address(compliance));
        assertEq(token.mintController(), address(minter));
        assertEq(token.burnController(), address(burner));
    }

    function test_mint_onlyController() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        vm.expectRevert(Errors.NotAuthorized.selector);
        token.mint(alice, 10 * 1e18, TR);
    }

    function test_mint_viaController_fullFlow() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);

        bytes32 proposalId =
            _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-001"));
        vm.prank(executor);
        minter.executeMint(proposalId);

        assertEq(token.balanceOf(alice), 100 * 1e18);
        assertEq(token.totalSupply(), 100 * 1e18);
    }

    function test_transfer_requiresBothSidesKyc() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);
        bytes32 proposalId =
            _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-002"));
        vm.prank(executor);
        minter.executeMint(proposalId);

        // Bob'un KYC'si yok → transfer fail
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(Errors.KycRequired.selector, bob));
        token.transfer(bob, 10 * 1e18);

        // Bob KYC alırsa başarılı
        _setKyc(bob, CH);
        vm.prank(alice);
        token.transfer(bob, 10 * 1e18);
        assertEq(token.balanceOf(bob), 10 * 1e18);
    }

    function test_transfer_blockedWhenFrozen() public {
        _setKyc(alice, TR);
        _setKyc(bob, TR);
        _publishReserve(1000 * 1e18);
        bytes32 pid =
            _proposeAndApproveMint(alice, 50 * 1e18, TR, keccak256("alloc-003"));
        vm.prank(executor);
        minter.executeMint(pid);

        vm.prank(complianceOfficer);
        compliance.freeze(alice, keccak256("court-order-42"));

        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(Errors.WalletFrozen.selector, alice));
        token.transfer(bob, 5 * 1e18);
    }

    function test_transfer_blockedWhenPaused() public {
        _setKyc(alice, TR);
        _setKyc(bob, TR);
        _publishReserve(1000 * 1e18);
        bytes32 pid =
            _proposeAndApproveMint(alice, 50 * 1e18, TR, keccak256("alloc-004"));
        vm.prank(executor);
        minter.executeMint(pid);

        vm.prank(pauser);
        token.pause();

        vm.prank(alice);
        vm.expectRevert();
        token.transfer(bob, 5 * 1e18);

        vm.prank(treasury);
        token.unpause();

        vm.prank(alice);
        token.transfer(bob, 5 * 1e18);
        assertEq(token.balanceOf(bob), 5 * 1e18);
    }

    function test_transfer_travelRule_blocksLargeTxUntilApproved() public {
        _setKyc(alice, TR);
        _setKyc(bob, CH);
        _publishReserve(100_000 * 1e18);

        bytes32 pid = _proposeAndApproveMint(
            alice, 5_000 * 1e18, TR, keccak256("alloc-big")
        );
        vm.prank(executor);
        minter.executeMint(pid);

        // 1500 gram (eşik 1000) → Travel Rule gerekli
        uint256 big = 1_500 * 1e18;

        vm.prank(alice);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.TravelRuleRequired.selector, alice, bob, big)
        );
        token.transfer(bob, big);

        // Counterparty onayı kaydedildiğinde transfer başarılı
        vm.prank(complianceOfficer);
        compliance.recordTravelRuleApproval(alice, bob, big, keccak256("ivms101-data"));

        vm.prank(alice);
        token.transfer(bob, big);
        assertEq(token.balanceOf(bob), big);
    }

    function test_burn_onlyController() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);
        bytes32 pid =
            _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-005"));
        vm.prank(executor);
        minter.executeMint(pid);

        vm.expectRevert(Errors.NotAuthorized.selector);
        token.burnFrom(alice, 10 * 1e18);
    }

    function test_roles_onlyTreasuryCanSetComplianceRegistry() public {
        address newReg = makeAddr("newReg");
        vm.expectRevert();
        token.setComplianceRegistry(newReg);

        vm.prank(treasury);
        token.setComplianceRegistry(newReg);
        assertEq(token.complianceRegistry(), newReg);
    }
}
