// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Script, console2 } from "forge-std/Script.sol";
import { UUPSUpgradeable } from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import { stdJson } from "forge-std/StdJson.sol";

import { GoldToken } from "../src/GoldToken.sol";
import { ComplianceRegistry } from "../src/ComplianceRegistry.sol";
import { MintController } from "../src/MintController.sol";
import { BurnController } from "../src/BurnController.sol";

/// @notice UUPS upgrade script — upgrades one or all proxy contracts to new implementations.
/// @dev    The deployer wallet must hold UPGRADER_ROLE on the target proxy.
///         Usage:
///           forge script script/Upgrade.s.sol --sig "upgradeGoldToken()" ...
///           forge script script/Upgrade.s.sol --sig "upgradeAll()" ...
///
///         Required env vars (in addition to DEPLOYER_PRIVATE_KEY):
///           GOLD_TOKEN_PROXY         — proxy address from deployments/<chainId>.json
///           COMPLIANCE_REGISTRY_PROXY
///           MINT_CONTROLLER_PROXY
///           BURN_CONTROLLER_PROXY
///
///         The script reads from the address registry automatically when
///         CHAIN_ID is set and the deployments/<chainId>.json file exists.
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

    // ──────────────────────────────────────────────────────────────────────
    // Individual upgrade targets
    // ──────────────────────────────────────────────────────────────────────

    function upgradeGoldToken() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("GOLD_TOKEN_PROXY", "goldToken");

        vm.startBroadcast(pk);
        GoldToken newImpl = new GoldToken();
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(address(newImpl), "");
        vm.stopBroadcast();

        console2.log("GoldToken upgraded");
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", address(newImpl));
    }

    function upgradeComplianceRegistry() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("COMPLIANCE_REGISTRY_PROXY", "complianceRegistry");

        vm.startBroadcast(pk);
        ComplianceRegistry newImpl = new ComplianceRegistry();
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(address(newImpl), "");
        vm.stopBroadcast();

        console2.log("ComplianceRegistry upgraded");
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", address(newImpl));
    }

    function upgradeMintController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("MINT_CONTROLLER_PROXY", "mintController");

        vm.startBroadcast(pk);
        MintController newImpl = new MintController();
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(address(newImpl), "");
        vm.stopBroadcast();

        console2.log("MintController upgraded");
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", address(newImpl));
    }

    function upgradeBurnController() external {
        (uint256 pk,) = _deployer();
        address proxyAddr = _proxy("BURN_CONTROLLER_PROXY", "burnController");

        vm.startBroadcast(pk);
        BurnController newImpl = new BurnController();
        UUPSUpgradeable(proxyAddr).upgradeToAndCall(address(newImpl), "");
        vm.stopBroadcast();

        console2.log("BurnController upgraded");
        console2.log("  proxy:   ", proxyAddr);
        console2.log("  new impl:", address(newImpl));
    }

    // ──────────────────────────────────────────────────────────────────────
    // Upgrade all UUPS proxies in one broadcast
    // ──────────────────────────────────────────────────────────────────────

    function upgradeAll() external {
        (uint256 pk,) = _deployer();

        address goldTokenProxy = _proxy("GOLD_TOKEN_PROXY", "goldToken");
        address complianceProxy = _proxy("COMPLIANCE_REGISTRY_PROXY", "complianceRegistry");
        address mintProxy = _proxy("MINT_CONTROLLER_PROXY", "mintController");
        address burnProxy = _proxy("BURN_CONTROLLER_PROXY", "burnController");

        vm.startBroadcast(pk);

        GoldToken newToken = new GoldToken();
        UUPSUpgradeable(goldTokenProxy).upgradeToAndCall(address(newToken), "");

        ComplianceRegistry newCompliance = new ComplianceRegistry();
        UUPSUpgradeable(complianceProxy).upgradeToAndCall(address(newCompliance), "");

        MintController newMint = new MintController();
        UUPSUpgradeable(mintProxy).upgradeToAndCall(address(newMint), "");

        BurnController newBurn = new BurnController();
        UUPSUpgradeable(burnProxy).upgradeToAndCall(address(newBurn), "");

        vm.stopBroadcast();

        console2.log("=== GOLD Full Upgrade ===");
        console2.log("GoldToken impl:          ", address(newToken));
        console2.log("ComplianceRegistry impl: ", address(newCompliance));
        console2.log("MintController impl:     ", address(newMint));
        console2.log("BurnController impl:     ", address(newBurn));
    }
}
