// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { Test } from "forge-std/Test.sol";
import { ERC1967Proxy } from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

import { GoldToken } from "../src/GoldToken.sol";
import { ComplianceRegistry } from "../src/ComplianceRegistry.sol";
import { ReserveOracle } from "../src/ReserveOracle.sol";
import { MintController } from "../src/MintController.sol";
import { BurnController } from "../src/BurnController.sol";
import { IComplianceRegistry } from "../src/interfaces/IComplianceRegistry.sol";
import { IReserveOracle } from "../src/interfaces/IReserveOracle.sol";
import { IMintController } from "../src/interfaces/IMintController.sol";

/// @notice Test ortak setup — aktörler, proxy deploy, roller.
abstract contract BaseTest is Test {
    // Aktörler
    address internal treasury = makeAddr("treasury");
    address internal pauser = makeAddr("pauser");
    address internal kycWriter = makeAddr("kycWriter");
    address internal complianceOfficer; // derived from complianceOfficerPk in setUp()
    address internal proposer = makeAddr("proposer");
    address internal executor = makeAddr("executor");
    address internal burnOperator = makeAddr("burnOperator");

    address[5] internal approvers = [
        makeAddr("approver1"),
        makeAddr("approver2"),
        makeAddr("approver3"),
        makeAddr("approver4"),
        makeAddr("approver5")
    ];

    // Test kullanıcıları
    address internal alice = makeAddr("alice");
    address internal bob = makeAddr("bob");
    address internal carol = makeAddr("carol");

    // Denetçi (imza için private key gerekli)
    uint256 internal auditorPk = 0xA11CE;
    address internal auditor;

    // Uyum Memuru (imza için private key gerekli)
    uint256 internal complianceOfficerPk = 0xC0FF1CE;
    // complianceOfficer address derived from complianceOfficerPk in setUp()

    // Sözleşmeler
    GoldToken internal token;
    ComplianceRegistry internal compliance;
    ReserveOracle internal oracle;
    MintController internal minter;
    BurnController internal burner;

    // Sabitler
    bytes2 internal constant TR = "TR";
    bytes2 internal constant CH = "CH";
    uint256 internal constant TRAVEL_RULE_THRESHOLD = 1000 * 1e18; // 1000 gram
    uint256 internal constant MAX_RESERVE_AGE = 35 days;
    uint8 internal constant APPROVAL_THRESHOLD = 3;

    function setUp() public virtual {
        auditor = vm.addr(auditorPk);
        complianceOfficer = vm.addr(complianceOfficerPk);

        // 1. ComplianceRegistry
        ComplianceRegistry regImpl = new ComplianceRegistry();
        bytes memory regInit = abi.encodeCall(
            ComplianceRegistry.initialize,
            (treasury, kycWriter, complianceOfficer, TRAVEL_RULE_THRESHOLD)
        );
        compliance = ComplianceRegistry(address(new ERC1967Proxy(address(regImpl), regInit)));

        // 2. Token
        GoldToken tokenImpl = new GoldToken();
        bytes memory tokenInit = abi.encodeCall(
            GoldToken.initialize,
            ("GOLD Gold", "GOLD", treasury, pauser, address(compliance))
        );
        token = GoldToken(address(new ERC1967Proxy(address(tokenImpl), tokenInit)));

        // 3. ReserveOracle (immutable deploy)
        address[] memory auditors = new address[](1);
        auditors[0] = auditor;
        oracle = new ReserveOracle(treasury, auditors);

        // 4. MintController
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
                APPROVAL_THRESHOLD,
                MAX_RESERVE_AGE
            )
        );
        minter = MintController(address(new ERC1967Proxy(address(mintImpl), mintInit)));

        // 5. BurnController
        BurnController burnImpl = new BurnController();
        bytes memory burnInit = abi.encodeCall(
            BurnController.initialize,
            (treasury, address(token), address(compliance), burnOperator, 1000 * 1e18)
        );
        burner = BurnController(address(new ERC1967Proxy(address(burnImpl), burnInit)));

        // 6. Token'a controller adreslerini ata
        vm.startPrank(treasury);
        token.setMintController(address(minter));
        token.setBurnController(address(burner));
        vm.stopPrank();
    }

    // ──────────────────────────────────────────────────────────────────────
    // Yardımcılar
    // ──────────────────────────────────────────────────────────────────────

    function _setKyc(address wallet, bytes2 jurisdiction) internal {
        IComplianceRegistry.WalletProfile memory p = IComplianceRegistry.WalletProfile({
            tier: IComplianceRegistry.KycTier.ENHANCED,
            jurisdiction: jurisdiction,
            kycApprovedAt: uint64(block.timestamp),
            kycExpiresAt: uint64(block.timestamp + 365 days),
            frozen: false,
            sanctioned: false
        });
        vm.prank(kycWriter);
        compliance.setProfile(wallet, p);
    }

    function _publishReserve(uint256 totalGrams) internal {
        IReserveOracle.Attestation memory a = IReserveOracle.Attestation({
            timestamp: uint64(block.timestamp),
            asOf: uint64(block.timestamp),
            totalGrams: totalGrams,
            merkleRoot: bytes32(uint256(0xAB)),
            ipfsCid: bytes32(uint256(0xCD)),
            auditor: auditor
        });

        bytes32 structHash = keccak256(
            abi.encode(
                keccak256(
                    "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
                ),
                a.timestamp,
                a.asOf,
                a.totalGrams,
                a.merkleRoot,
                a.ipfsCid,
                a.auditor
            )
        );
        bytes32 domain = oracle.DOMAIN_SEPARATOR();
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", domain, structHash));
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(auditorPk, digest);
        bytes memory sig = abi.encodePacked(r, s, v);

        oracle.publish(a, sig);
    }

    function _proposeAndApproveMint(
        address to,
        uint256 amount,
        bytes2 jurisdiction,
        bytes32 allocationId
    ) internal returns (bytes32 proposalId) {
        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar-001");

        vm.prank(proposer);
        proposalId = minter.proposeMint(
            _mintRequest(to, amount, allocationId, bars, jurisdiction)
        );

        for (uint256 i = 0; i < APPROVAL_THRESHOLD; i++) {
            vm.prank(approvers[i]);
            minter.approveMint(proposalId);
        }
    }

    function _mintRequest(
        address to,
        uint256 amount,
        bytes32 allocationId,
        bytes32[] memory bars,
        bytes2 jurisdiction
    ) internal view returns (IMintController.MintRequest memory) {
        return IMintController.MintRequest({
            to: to,
            amount: amount,
            allocationId: allocationId,
            barSerials: bars,
            jurisdiction: jurisdiction,
            proposedAt: uint64(block.timestamp)
        });
    }
}
