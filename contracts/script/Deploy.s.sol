// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Script, console2 } from "forge-std/Script.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

import { GoldToken } from "../src/GoldToken.sol";
import { ComplianceRegistry } from "../src/ComplianceRegistry.sol";
import { ReserveOracle } from "../src/ReserveOracle.sol";
import { MintController } from "../src/MintController.sol";
import { BurnController } from "../src/BurnController.sol";

/// @notice Testnet dağıtım script'i.
/// @dev Mainnet öncesi: Treasury Safe gerçek 3/5 çoklu-imza olmalı.
///      Bu script dev/staging için tek EOA ile çalışır.
contract Deploy is Script {
    function run() external {
        uint256 deployerPk = vm.envUint("DEPLOYER_PRIVATE_KEY");
        address treasury = vm.envAddress("TREASURY_ADDRESS");
        address pauser = vm.envAddress("PAUSER_ADDRESS");
        address kycWriter = vm.envAddress("KYC_WRITER_ADDRESS");
        address complianceOfficer = vm.envAddress("COMPLIANCE_OFFICER_ADDRESS");
        address proposer = vm.envAddress("MINT_PROPOSER_ADDRESS");
        address executor = vm.envAddress("MINT_EXECUTOR_ADDRESS");
        address burnOperator = vm.envAddress("BURN_OPERATOR_ADDRESS");
        address auditor = vm.envAddress("AUDITOR_ADDRESS");

        address[5] memory approvers;
        approvers[0] = vm.envAddress("APPROVER_1");
        approvers[1] = vm.envAddress("APPROVER_2");
        approvers[2] = vm.envAddress("APPROVER_3");
        approvers[3] = vm.envAddress("APPROVER_4");
        approvers[4] = vm.envAddress("APPROVER_5");

        vm.startBroadcast(deployerPk);

        // 1. ComplianceRegistry (UUPS)
        ComplianceRegistry regImpl = new ComplianceRegistry();
        bytes memory regInit = abi.encodeCall(
            ComplianceRegistry.initialize,
            (treasury, kycWriter, complianceOfficer, 1000 * 1e18)
        );
        ERC1967Proxy regProxy = new ERC1967Proxy(address(regImpl), regInit);
        ComplianceRegistry compliance = ComplianceRegistry(address(regProxy));

        // 2. GoldToken (UUPS)
        GoldToken tokenImpl = new GoldToken();
        bytes memory tokenInit = abi.encodeCall(
            GoldToken.initialize,
            ("GOLD Gold", "GOLD", treasury, pauser, address(compliance))
        );
        ERC1967Proxy tokenProxy = new ERC1967Proxy(address(tokenImpl), tokenInit);
        GoldToken token = GoldToken(address(tokenProxy));

        // 3. ReserveOracle (immutable)
        address[] memory initialAuditors = new address[](1);
        initialAuditors[0] = auditor;
        ReserveOracle oracle = new ReserveOracle(treasury, initialAuditors);

        // 4. MintController (UUPS)
        MintController mintImpl = new MintController();
        address[] memory approverList = new address[](5);
        for (uint256 i = 0; i < 5; i++) approverList[i] = approvers[i];
        bytes memory mintInit = abi.encodeCall(
            MintController.initialize,
            (
                treasury,
                address(token),
                address(compliance),
                address(oracle),
                approverList,
                proposer,
                executor,
                3, // 3/5 approval
                35 days
            )
        );
        ERC1967Proxy mintProxy = new ERC1967Proxy(address(mintImpl), mintInit);
        MintController minter = MintController(address(mintProxy));

        // 5. BurnController (UUPS)
        BurnController burnImpl = new BurnController();
        bytes memory burnInit = abi.encodeCall(
            BurnController.initialize,
            (treasury, address(token), address(compliance), burnOperator, 1000 * 1e18)
        );
        ERC1967Proxy burnProxy = new ERC1967Proxy(address(burnImpl), burnInit);
        BurnController burner = BurnController(address(burnProxy));

        // 6. Token controller'larını bağla
        token.setMintController(address(minter));
        token.setBurnController(address(burner));

        vm.stopBroadcast();

        console2.log("=== GOLD Deployment ===");
        console2.log("ComplianceRegistry:", address(compliance));
        console2.log("GoldToken:", address(token));
        console2.log("ReserveOracle:", address(oracle));
        console2.log("MintController:", address(minter));
        console2.log("BurnController:", address(burner));
        console2.log("Treasury:", treasury);
    }
}
