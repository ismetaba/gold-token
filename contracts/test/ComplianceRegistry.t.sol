// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { BaseTest } from "./Base.t.sol";
import { IComplianceRegistry } from "../src/interfaces/IComplianceRegistry.sol";

contract ComplianceRegistryTest is BaseTest {
    // ──────────────────────────────────────────────────────────────────────
    // canBurn — KYC checks (MEDIUM-3)
    // ──────────────────────────────────────────────────────────────────────

    function test_canBurn_requiresKyc() public view {
        // alice has no KYC profile — canBurn should return false
        assertFalse(compliance.canBurn(alice, 1 * 1e18), "canBurn without KYC");
    }

    function test_canBurn_validKycPasses() public {
        _setKyc(alice, TR);
        assertTrue(compliance.canBurn(alice, 1 * 1e18), "canBurn with valid KYC");
    }

    function test_canBurn_expiredKycFails() public {
        // Set KYC that will expire
        IComplianceRegistry.WalletProfile memory p = IComplianceRegistry.WalletProfile({
            tier: IComplianceRegistry.KycTier.ENHANCED,
            jurisdiction: TR,
            kycApprovedAt: uint64(block.timestamp),
            kycExpiresAt: uint64(block.timestamp + 1 days),
            frozen: false,
            sanctioned: false
        });
        vm.prank(kycWriter);
        compliance.setProfile(alice, p);

        // Before expiry — should pass
        assertTrue(compliance.canBurn(alice, 1 * 1e18), "canBurn before expiry");

        // After expiry — should fail
        vm.warp(block.timestamp + 2 days);
        assertFalse(compliance.canBurn(alice, 1 * 1e18), "canBurn after expiry");
    }

    function test_canBurn_frozenFails() public {
        _setKyc(alice, TR);
        vm.prank(complianceOfficer);
        compliance.freeze(alice, keccak256("test"));
        assertFalse(compliance.canBurn(alice, 1 * 1e18), "canBurn when frozen");
    }

    function test_canBurn_sanctionedFails() public {
        _setKyc(alice, TR);
        vm.prank(complianceOfficer);
        compliance.setSanctioned(alice, true);
        assertFalse(compliance.canBurn(alice, 1 * 1e18), "canBurn when sanctioned");
    }

    // ──────────────────────────────────────────────────────────────────────
    // canTransfer — basic coverage
    // ──────────────────────────────────────────────────────────────────────

    function test_canTransfer_validProfilesPasses() public {
        _setKyc(alice, TR);
        _setKyc(bob, TR);
        assertTrue(compliance.canTransfer(alice, bob, 100 * 1e18), "valid transfer");
    }

    function test_canTransfer_noKycFails() public {
        _setKyc(alice, TR);
        // bob has no KYC
        assertFalse(compliance.canTransfer(alice, bob, 100 * 1e18), "no KYC recipient");
    }

    // ──────────────────────────────────────────────────────────────────────
    // isKycValid
    // ──────────────────────────────────────────────────────────────────────

    function test_isKycValid_returnsFalseWhenNone() public view {
        assertFalse(compliance.isKycValid(alice), "no profile");
    }

    function test_isKycValid_returnsTrueWhenValid() public {
        _setKyc(alice, TR);
        assertTrue(compliance.isKycValid(alice), "valid profile");
    }
}
