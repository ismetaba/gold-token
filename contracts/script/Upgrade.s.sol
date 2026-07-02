// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Script, console2 } from "forge-std/Script.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { stdJson } from "forge-std/StdJson.sol";

import { GoldToken } from "../src/GoldToken.sol";
import { ComplianceRegistry } from "../src/ComplianceRegistry.sol";
import { MintController } from "../src/MintController.sol";
import { BurnController } from "../src/BurnController.sol";

/// @notice UUPS upgrade script. Upgrades are TIMELOCKED on-chain, so the flow
///         is two-phase and split into separate script entrypoints:
///
///           Phase 1 (schedule):  forge script ... --sig "scheduleGoldToken()"
///             - deploys the new implementation
///             - calls scheduleUpgrade(newImpl) on the proxy (needs UPGRADER_ROLE)
///             - prints the new impl address — RECORD IT.
///
///           Phase 2 (apply, after `upgradeDelay`, default 48h):
///                                 GOLD_TOKEN_NEW_IMPL=0x... forge script ... \
///                                   --sig "applyGoldToken()"
///             - calls upgradeToAndCall(newImpl, "") with the previously
///               scheduled implementation address (read from *_NEW_IMPL env).
///
///         Applying before the delay elapses (or with a different impl than was
///         scheduled) reverts with UpgradeTimelockActive / UpgradeNotTimelocked.
///
///         Required env vars (in addition to DEPLOYER_PRIVATE_KEY):
///           GOLD_TOKEN_PROXY / COMPLIANCE_REGISTRY_PROXY /
///           MINT_CONTROLLER_PROXY / BURN_CONTROLLER_PROXY  (or the registry)
///         For the apply phase, the matching *_NEW_IMPL address from phase 1.
///
///         ⚠ AUTHORITY: schedule/apply require UPGRADER_ROLE. This EOA-broadcast flow
///         only works when DEPLOYER_PRIVATE_KEY *holds* UPGRADER_ROLE — i.e. a dev/test
///         setup where the deployer is also the treasury/upgrader. In the production
///         topology UPGRADER_ROLE is held by a separate TimelockController / Gnosis Safe
///         (see Deploy.s.sol's UPGRADER_ADDRESS), which this EOA script cannot drive.
///         For that case, deploy the new implementation permissionlessly and submit the
///         scheduleUpgrade(newImpl) / upgradeToAndCall(newImpl, "") calldata through the
///         Safe/timelock instead of broadcasting here.
contract Upgrade is Script {
    using stdJson for string;

    // ──────────────────────────────────────────────────────────────────────
    // Helpers
    // ──────────────────────────────────────────────────────────────────────

    function _deployer() internal returns (uint256 pk, address addr) {
        pk = vm.envUint("DEPLOYER_PRIVATE_KEY");
        addr = vm.addr(pk);
    }

    /// @dev Read proxy address: env var overrides, then registry fallback.
    function _proxy(string memory envKey, string memory registryKey) internal view returns (address) {
        address fromEnv = vm.envOr(envKey, address(0));
        if (fromEnv != address(0)) return fromEnv;

        // Fallback: read from address registry
        string memory chainIdStr = vm.toString(block.chainid);
        string memory path = string.concat("./deployments/", chainIdStr, ".json");
        string memory json = vm.readFile(path);
        return json.readAddress(string.concat(".", registryKey));
    }

    /// @dev The scheduled implementation address for the apply phase.
    function _newImpl(string memory envKey) internal view returns (address) {
        address impl = vm.envOr(envKey, address(0));
        require(impl != address(0), string.concat(envKey, " must be set to the scheduled implementation"));
        return impl;
    }

    // ──────────────────────────────────────────────────────────────────────
    // Phase 1: schedule
    // ──────────────────────────────────────────────────────────────────────

    function scheduleGoldToken() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("GOLD_TOKEN_PROXY", "goldToken");

        vm.startBroadcast(pk);
        GoldToken newImpl = new GoldToken();
        GoldToken(proxyAddr).scheduleUpgrade(address(newImpl));
        vm.stopBroadcast();

        _logScheduled("GoldToken", proxyAddr, address(newImpl), "GOLD_TOKEN_NEW_IMPL");
    }

    function scheduleComplianceRegistry() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("COMPLIANCE_REGISTRY_PROXY", "complianceRegistry");

        vm.startBroadcast(pk);
        ComplianceRegistry newImpl = new ComplianceRegistry();
        ComplianceRegistry(proxyAddr).scheduleUpgrade(address(newImpl));
        vm.stopBroadcast();

        _logScheduled("ComplianceRegistry", proxyAddr, address(newImpl), "COMPLIANCE_REGISTRY_NEW_IMPL");
    }

    function scheduleMintController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("MINT_CONTROLLER_PROXY", "mintController");

        vm.startBroadcast(pk);
        MintController newImpl = new MintController();
        MintController(proxyAddr).scheduleUpgrade(address(newImpl));
        vm.stopBroadcast();

        _logScheduled("MintController", proxyAddr, address(newImpl), "MINT_CONTROLLER_NEW_IMPL");
    }

    function scheduleBurnController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("BURN_CONTROLLER_PROXY", "burnController");

        vm.startBroadcast(pk);
        BurnController newImpl = new BurnController();
        BurnController(proxyAddr).scheduleUpgrade(address(newImpl));
        vm.stopBroadcast();

        _logScheduled("BurnController", proxyAddr, address(newImpl), "BURN_CONTROLLER_NEW_IMPL");
    }

    /// @notice Schedule all four proxies in one broadcast.
    function scheduleAll() external {
        (uint256 pk,) = _deployer();
        address goldTokenProxy = _proxy("GOLD_TOKEN_PROXY", "goldToken");
        address complianceProxy = _proxy("COMPLIANCE_REGISTRY_PROXY", "complianceRegistry");
        address mintProxy = _proxy("MINT_CONTROLLER_PROXY", "mintController");
        address burnProxy = _proxy("BURN_CONTROLLER_PROXY", "burnController");

        vm.startBroadcast(pk);
        address t = address(new GoldToken());
        GoldToken(goldTokenProxy).scheduleUpgrade(t);
        address c = address(new ComplianceRegistry());
        ComplianceRegistry(complianceProxy).scheduleUpgrade(c);
        address m = address(new MintController());
        MintController(mintProxy).scheduleUpgrade(m);
        address b = address(new BurnController());
        BurnController(burnProxy).scheduleUpgrade(b);
        vm.stopBroadcast();

        console2.log("=== GOLD Full Upgrade SCHEDULED (apply after upgradeDelay) ===");
        _logScheduled("GoldToken", goldTokenProxy, t, "GOLD_TOKEN_NEW_IMPL");
        _logScheduled("ComplianceRegistry", complianceProxy, c, "COMPLIANCE_REGISTRY_NEW_IMPL");
        _logScheduled("MintController", mintProxy, m, "MINT_CONTROLLER_NEW_IMPL");
        _logScheduled("BurnController", burnProxy, b, "BURN_CONTROLLER_NEW_IMPL");
    }

    // ──────────────────────────────────────────────────────────────────────
    // Phase 2: apply (after the timelock delay)
    // ──────────────────────────────────────────────────────────────────────

    function applyGoldToken() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("GOLD_TOKEN_PROXY", "goldToken");
        address impl = _newImpl("GOLD_TOKEN_NEW_IMPL");

        vm.startBroadcast(pk);
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(impl, "");
        vm.stopBroadcast();
        _logApplied("GoldToken", proxyAddr, impl);
    }

    function applyComplianceRegistry() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("COMPLIANCE_REGISTRY_PROXY", "complianceRegistry");
        address impl = _newImpl("COMPLIANCE_REGISTRY_NEW_IMPL");

        vm.startBroadcast(pk);
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(impl, "");
        vm.stopBroadcast();
        _logApplied("ComplianceRegistry", proxyAddr, impl);
    }

    function applyMintController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("MINT_CONTROLLER_PROXY", "mintController");
        address impl = _newImpl("MINT_CONTROLLER_NEW_IMPL");

        vm.startBroadcast(pk);
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(impl, "");
        vm.stopBroadcast();
        _logApplied("MintController", proxyAddr, impl);
    }

    function applyBurnController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("BURN_CONTROLLER_PROXY", "burnController");
        address impl = _newImpl("BURN_CONTROLLER_NEW_IMPL");

        vm.startBroadcast(pk);
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(impl, "");
        vm.stopBroadcast();
        _logApplied("BurnController", proxyAddr, impl);
    }

    // ──────────────────────────────────────────────────────────────────────
    // Logging
    // ──────────────────────────────────────────────────────────────────────

    function _logScheduled(string memory name, address proxyAddr, address impl, string memory envKey) internal pure {
        console2.log(string.concat(name, " upgrade scheduled"));
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", impl);
        console2.log(string.concat("  -> after the delay, set ", envKey, "=<new impl> and run apply", name, "()"));
    }

    function _logApplied(string memory name, address proxyAddr, address impl) internal pure {
        console2.log(string.concat(name, " upgraded"));
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", impl);
    }
}
