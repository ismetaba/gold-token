// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { Errors } from "../src/libraries/Errors.sol";
import { GoldToken } from "../src/GoldToken.sol";
import { IComplianceRegistry } from "../src/interfaces/IComplianceRegistry.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

/// @notice Regression tests for the security fixes applied during the audit remediation.
///         Each test pins a specific finding so a regression re-surfaces as a failure.
contract RemediationTest is BaseTest {
    bytes2 internal constant XX = "XX";

    function _mintTo(address to, uint256 gross, bytes2 jur, bytes32 alloc) internal {
        bytes32 pid = _proposeAndApproveMint(to, gross, jur, alloc);
        vm.prank(executor);
        minter.executeMint(pid);
    }

    // ── Travel Rule approval is single-use (HIGH) ─────────────────────────────

    function test_travelRule_approvalIsSingleUse() public {
        _setKyc(alice, TR);
        _setKyc(bob, CH);
        _publishReserve(100_000 * 1e18);
        _mintTo(alice, 10_000 * 1e18, TR, keccak256("alloc-tr"));

        uint256 big = 1_500 * 1e18; // above the 1_000 travel-rule threshold

        // One IVMS-101 approval authorises exactly one above-threshold transfer.
        vm.prank(complianceOfficer);
        compliance.recordTravelRuleApproval(alice, bob, big, keccak256("ivms101"));

        vm.prank(alice);
        token.transfer(bob, big);
        assertEq(token.balanceOf(bob), big);

        // A second identical transfer must be re-gated (approval was consumed).
        vm.prank(alice);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.TravelRuleRequired.selector, alice, bob, big)
        );
        token.transfer(bob, big);
    }

    function test_screenTransfer_onlyToken() public {
        _setKyc(alice, TR);
        _setKyc(bob, TR);
        // A caller other than the registered token cannot invoke screenTransfer.
        vm.expectRevert(Errors.NotAuthorized.selector);
        compliance.screenTransfer(alice, bob, 1);
    }

    // ── setProfile cannot clear compliance flags (HIGH) ───────────────────────

    function test_setProfile_preservesFrozenAndSanctioned() public {
        _setKyc(alice, TR);

        vm.startPrank(complianceOfficer);
        compliance.freeze(alice, keccak256("court-order"));
        compliance.setSanctioned(alice, true);
        vm.stopPrank();

        // KYC_WRITER re-writes the whole profile with default (false) flags.
        IComplianceRegistry.WalletProfile memory p = IComplianceRegistry.WalletProfile({
            tier: IComplianceRegistry.KycTier.ENHANCED,
            jurisdiction: TR,
            kycApprovedAt: uint64(block.timestamp),
            kycExpiresAt: uint64(block.timestamp + 365 days),
            frozen: false,
            sanctioned: false
        });
        vm.prank(kycWriter);
        compliance.setProfile(alice, p);

        // The compliance-officer-controlled flags must survive the overwrite.
        assertTrue(compliance.isFrozen(alice), "freeze cleared by KYC writer");
        assertTrue(compliance.isSanctioned(alice), "sanction cleared by KYC writer");
    }

    // ── Upgrade timelock cannot be shortened after scheduling (HIGH) ──────────

    function test_setUpgradeDelay_belowMinimumReverts() public {
        vm.prank(treasury);
        vm.expectRevert(
            abi.encodeWithSelector(Errors.UpgradeDelayBelowMinimum.selector, 1 hours, 24 hours)
        );
        token.setUpgradeDelay(1 hours);
    }

    function test_upgradeTimelock_cannotBeShortenedAfterSchedule() public {
        GoldToken newImpl = new GoldToken();

        // Schedule with the default 48h delay: eligibility snapshot = now + 48h.
        vm.prank(treasury);
        token.scheduleUpgrade(address(newImpl));

        // Reduce the delay to the floor (24h) AFTER scheduling.
        vm.prank(treasury);
        token.setUpgradeDelay(24 hours);

        // 30h later: past the NEW 24h delay, but before the 48h snapshot → still blocked,
        // proving the eligibility is read from the schedule-time snapshot.
        vm.warp(block.timestamp + 30 hours);
        vm.prank(treasury);
        vm.expectRevert(); // UpgradeTimelockActive(scheduledEligibleAt)
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");

        // Past the original 48h window → eligible.
        vm.warp(block.timestamp + 24 hours);
        vm.prank(treasury);
        UUPSUpgradeable(address(token)).upgradeToAndCall(address(newImpl), "");
        assertEq(token.symbol(), "GOLD");
    }

    // ── canMint gates on the recipient's stored jurisdiction (MEDIUM) ─────────

    function test_canMint_gatesOnStoredJurisdiction() public {
        // Alice's KYC-verified jurisdiction is XX, which the treasury blocks.
        _setKyc(alice, XX);
        vm.prank(treasury);
        compliance.setJurisdictionBlocked(XX, true);

        // A proposer-supplied unblocked provenance label must NOT bypass the block.
        assertFalse(compliance.canMint(alice, 100 * 1e18, TR), "blocked stored jurisdiction");
    }

    // ── Rate-limit misconfiguration is rejected (MEDIUM) ──────────────────────

    function test_setRateLimit_rejectsZeroWindowWithMax() public {
        vm.prank(treasury);
        vm.expectRevert(abi.encodeWithSelector(Errors.InvalidRateLimit.selector, 0, 100));
        minter.setRateLimit(0, 100);
    }

    // ── Mint fee recipient is compliance-gated (LOW) ──────────────────────────

    function test_mintFee_revertsWhenFeeRecipientNotCompliant() public {
        _setKyc(alice, TR);
        _publishReserve(100_000 * 1e18);

        // Point the fee recipient at an address with no KYC profile.
        vm.prank(treasury);
        minter.setFeeRecipient(bob);

        bytes32 pid = _proposeAndApproveMint(alice, 1_000 * 1e18, TR, keccak256("alloc-fee"));
        vm.prank(executor);
        vm.expectRevert(Errors.NotAuthorized.selector);
        minter.executeMint(pid);
    }

    // ── Jurisdiction-blocked transfer reverts with a specific reason (LOW) ────

    function test_transfer_jurisdictionBlocked_specificRevert() public {
        _setKyc(alice, TR);
        _setKyc(bob, CH);
        _publishReserve(100_000 * 1e18);
        _mintTo(alice, 1_000 * 1e18, TR, keccak256("alloc-jur"));

        // Block CH after alice is funded; a small (sub-threshold) transfer to bob (CH)
        // should now revert with the jurisdiction-specific error, not generic NotAuthorized.
        vm.prank(treasury);
        compliance.setJurisdictionBlocked(CH, true);

        vm.prank(alice);
        vm.expectRevert(abi.encodeWithSelector(Errors.JurisdictionBlocked.selector, CH));
        token.transfer(bob, 10 * 1e18);
    }

    // ── setToken access control ───────────────────────────────────────────────

    function test_setToken_onlyTreasury() public {
        vm.prank(alice);
        vm.expectRevert();
        compliance.setToken(address(token));
    }
}
