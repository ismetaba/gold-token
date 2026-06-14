// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { GoldToken } from "../src/GoldToken.sol";
import { Roles } from "../src/libraries/Roles.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { IAccessControl } from "@openzeppelin/contracts/access/IAccessControl.sol";

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

        uint256 gross = 100 * 1e18;
        bytes32 proposalId =
            _proposeAndApproveMint(alice, gross, TR, keccak256("alloc-001"));
        vm.prank(executor);
        minter.executeMint(proposalId);

        // Alice receives gross minus the 0.25% mint fee; treasury receives the fee.
        // Total supply equals gross (fee tokens remain in circulation as treasury balance).
        assertEq(token.balanceOf(alice), _netMintAmount(gross));
        assertEq(token.totalSupply(), gross);
    }

    function test_transfer_requiresBothSidesKyc() public {
        _setKyc(alice, TR);
        _publishReserve(1000 * 1e18);
        bytes32 proposalId =
            _proposeAndApproveMint(alice, 100 * 1e18, TR, keccak256("alloc-002"));
        vm.prank(executor);
        minter.executeMint(proposalId);

        // Bob has no KYC — transfer must revert
        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(Errors.KycRequired.selector, bob));
        token.transfer(bob, 10 * 1e18);

        // After Bob is KYC-approved the transfer succeeds
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

        // 1 500 grams (threshold 1 000) → Travel Rule required
        uint256 big = 1_500 * 1e18;

        vm.prank(alice);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.TravelRuleRequired.selector, alice, bob, big)
        );
        token.transfer(bob, big);

        // Once counterparty approval is recorded the transfer succeeds
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

    // ──────────────────────────────────────────────────────────────────────
    // Upgrade timelock
    // ──────────────────────────────────────────────────────────────────────

    function test_upgradeTimelock_defaultDelayIs48h() public view {
        assertEq(token.upgradeDelay(), 48 hours);
    }

    function test_upgradeTimelock_immediateUpgradeReverts() public {
        GoldToken newImpl = new GoldToken();

        // Not scheduled at all → UpgradeNotTimelocked.
        vm.prank(treasury);
        vm.expectRevert(Errors.UpgradeNotTimelocked.selector);
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");
    }

    function test_upgradeTimelock_scheduledButTooEarlyReverts() public {
        GoldToken newImpl = new GoldToken();

        vm.prank(treasury); // treasury holds UPGRADER_ROLE
        token.scheduleUpgrade(address(newImpl));

        uint256 eligibleAt = block.timestamp + 48 hours;
        // Warp not far enough.
        vm.warp(block.timestamp + 47 hours);

        vm.prank(treasury);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.UpgradeTimelockActive.selector, eligibleAt)
        );
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");
    }

    function test_upgradeTimelock_succeedsAfterDelay() public {
        GoldToken newImpl = new GoldToken();

        vm.prank(treasury);
        token.scheduleUpgrade(address(newImpl));
        (address scheduled,) = token.scheduledUpgrade();
        assertEq(scheduled, address(newImpl));

        vm.warp(block.timestamp + 48 hours);

        vm.prank(treasury);
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");

        // Schedule slot consumed.
        (address afterImpl, uint256 afterAt) = token.scheduledUpgrade();
        assertEq(afterImpl, address(0));
        assertEq(afterAt, 0);
        // Token still functions post-upgrade.
        assertEq(token.symbol(), "GOLD");
    }

    function test_upgradeTimelock_cannotApplyDifferentImpl() public {
        GoldToken scheduledImpl = new GoldToken();
        GoldToken otherImpl = new GoldToken();

        vm.prank(treasury);
        token.scheduleUpgrade(address(scheduledImpl));
        vm.warp(block.timestamp + 48 hours);

        // Delay elapsed but the target differs from the scheduled impl.
        vm.prank(treasury);
        vm.expectRevert(Errors.UpgradeNotTimelocked.selector);
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(otherImpl), "");
    }

    function test_upgradeTimelock_nonUpgraderCannotSchedule() public {
        GoldToken newImpl = new GoldToken();
        // alice has no UPGRADER_ROLE.
        vm.prank(alice);
        vm.expectRevert(
            abi.encodeWithSelector(
                IAccessControl.AccessControlUnauthorizedAccount.selector,
                alice,
                Roles.UPGRADER_ROLE
            )
        );
        token.scheduleUpgrade(address(newImpl));
    }

    function test_upgradeTimelock_cancelScheduledUpgrade() public {
        GoldToken newImpl = new GoldToken();
        vm.prank(treasury);
        token.scheduleUpgrade(address(newImpl));

        vm.prank(treasury);
        token.cancelScheduledUpgrade();
        (address impl, uint256 at) = token.scheduledUpgrade();
        assertEq(impl, address(0));
        assertEq(at, 0);

        vm.warp(block.timestamp + 48 hours);
        vm.prank(treasury);
        vm.expectRevert(Errors.UpgradeNotTimelocked.selector);
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");
    }
}
