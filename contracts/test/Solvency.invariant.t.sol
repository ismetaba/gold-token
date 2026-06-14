// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { StdInvariant } from "forge-std/StdInvariant.sol";
import { BaseTest } from "./Base.t.sol";
import { IReserveOracle } from "../src/interfaces/IReserveOracle.sol";
import { IMintController } from "../src/interfaces/IMintController.sol";
import { IBurnController } from "../src/interfaces/IBurnController.sol";
import { GoldToken } from "../src/GoldToken.sol";
import { ComplianceRegistry } from "../src/ComplianceRegistry.sol";
import { ReserveOracle } from "../src/ReserveOracle.sol";
import { MintController } from "../src/MintController.sol";
import { BurnController } from "../src/BurnController.sol";

/// @notice Stateful handler that drives the GOLD system through realistic mint/burn/attest
///         actions so the fuzzer can probe the core solvency invariant.
contract SolvencyHandler {
    GoldToken internal token;
    ReserveOracle internal oracle;
    MintController internal minter;
    BurnController internal burner;
    ComplianceRegistry internal compliance;

    address internal treasury;
    address internal proposer;
    address internal executor;
    address internal burnOperator;
    address[] internal approvers;
    address internal user;

    uint256 internal auditorPk;
    uint256 internal auditor2Pk;
    address internal auditor;

    uint256 internal allocNonce;

    constructor(
        GoldToken _token,
        ReserveOracle _oracle,
        MintController _minter,
        BurnController _burner,
        ComplianceRegistry _compliance,
        address _treasury,
        address _proposer,
        address _executor,
        address _burnOperator,
        address[] memory _approvers,
        address _user,
        uint256 _auditorPk,
        uint256 _auditor2Pk,
        address _auditor
    ) {
        token = _token;
        oracle = _oracle;
        minter = _minter;
        burner = _burner;
        compliance = _compliance;
        treasury = _treasury;
        proposer = _proposer;
        executor = _executor;
        burnOperator = _burnOperator;
        approvers = _approvers;
        user = _user;
        auditorPk = _auditorPk;
        auditor2Pk = _auditor2Pk;
        auditor = _auditor;
    }

    Vm internal constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    /// @dev Publish a fresh attestation; totalGrams grows within the allowed cap.
    function attest(uint96 deltaGrams) external {
        uint256 prev = oracle.latestAttestedGrams();
        // Cap growth at +40% to stay within the oracle's +50% guard; allow first attest.
        uint256 maxDelta = prev == 0 ? 1_000_000 * 1e18 : (prev * 4000) / 10_000;
        uint256 delta = bound(uint256(deltaGrams), 0, maxDelta);
        uint256 newTotal = prev + delta;
        if (newTotal == 0) newTotal = 1e18;

        // Strictly-increasing timestamps.
        vm.warp(block.timestamp + 1 days);

        IReserveOracle.Attestation memory a = IReserveOracle.Attestation({
            timestamp: uint64(block.timestamp),
            asOf: uint64(block.timestamp),
            totalGrams: newTotal,
            merkleRoot: bytes32(uint256(1)),
            ipfsCid: bytes32(uint256(2)),
            auditor: auditor
        });
        bytes32 structHash = keccak256(
            abi.encode(
                keccak256(
                    "Attestation(uint64 timestamp,uint64 asOf,uint256 totalGrams,bytes32 merkleRoot,bytes32 ipfsCid,address auditor)"
                ),
                a.timestamp, a.asOf, a.totalGrams, a.merkleRoot, a.ipfsCid, a.auditor
            )
        );
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", oracle.DOMAIN_SEPARATOR(), structHash));
        bytes[] memory sigs = new bytes[](2);
        (uint8 v1, bytes32 r1, bytes32 s1) = vm.sign(auditorPk, digest);
        sigs[0] = abi.encodePacked(r1, s1, v1);
        (uint8 v2, bytes32 r2, bytes32 s2) = vm.sign(auditor2Pk, digest);
        sigs[1] = abi.encodePacked(r2, s2, v2);
        oracle.publish(a, sigs);
    }

    /// @dev Full mint flow for a bounded amount to `user`.
    function mint(uint96 amount) external {
        uint256 attested = oracle.latestAttestedGrams();
        if (attested == 0) return;
        uint256 supply = token.totalSupply();
        if (supply >= attested) return;
        uint256 headroom = attested - supply;
        uint256 amt = bound(uint256(amount), 0, headroom);
        if (amt == 0) return;

        bytes32 allocId = keccak256(abi.encode("alloc", allocNonce++));
        bytes32[] memory bars = new bytes32[](1);
        bars[0] = keccak256("bar");
        IMintController.MintRequest memory req = IMintController.MintRequest({
            to: user,
            amount: amt,
            allocationId: allocId,
            barSerials: bars,
            jurisdiction: "TR",
            proposedAt: uint64(block.timestamp)
        });

        vm.prank(proposer);
        bytes32 pid = minter.proposeMint(req);
        for (uint256 i = 0; i < 3; i++) {
            vm.prank(approvers[i]);
            minter.approveMint(pid);
        }
        vm.prank(executor);
        minter.executeMint(pid);
    }

    /// @dev Burn some of the user's balance via redemption (reduces supply).
    function burn(uint96 amount) external {
        uint256 bal = token.balanceOf(user);
        if (bal == 0) return;
        uint256 amt = bound(uint256(amount), 1, bal);

        vm.prank(user);
        token.approve(address(burner), amt);

        IBurnController.RedemptionRequest memory req = IBurnController.RedemptionRequest({
            from: user,
            amount: amt,
            redemptionType: IBurnController.RedemptionType.CASH_BACK,
            offChainOrderId: keccak256(abi.encode("order", allocNonce++)),
            deliveryRef: "ref"
        });
        vm.prank(burnOperator);
        burner.requestRedemption(req);
    }

    // bound helper (mirror of forge-std)
    function bound(uint256 x, uint256 min, uint256 max) internal pure returns (uint256) {
        if (min > max) return min;
        uint256 size = max - min + 1;
        if (size == 0) return min;
        return min + (x % size);
    }
}

interface Vm {
    function warp(uint256) external;
    function prank(address) external;
    function sign(uint256, bytes32) external pure returns (uint8, bytes32, bytes32);
}

contract SolvencyInvariantTest is StdInvariant, BaseTest {
    SolvencyHandler internal handler;

    function setUp() public override {
        super.setUp();

        // KYC the invariant user and the treasury (treasury receives mint fees).
        _setKyc(alice, TR);
        _setKyc(treasury, TR);

        // Higher reserve growth cap so the handler can grow reserves freely.
        vm.prank(treasury);
        oracle.setMaxGrowthBps(5000);

        address[] memory approverList = new address[](5);
        for (uint256 i = 0; i < 5; i++) approverList[i] = approvers[i];

        handler = new SolvencyHandler(
            token, oracle, minter, burner, compliance,
            treasury, proposer, executor, burnOperator,
            approverList, alice, auditorPk, auditor2Pk, auditor
        );

        targetContract(address(handler));
    }

    /// @notice CORE SOLVENCY: circulating supply can never exceed attested reserves.
    function invariant_supplyNeverExceedsAttestedReserves() public view {
        assertLe(token.totalSupply(), oracle.latestAttestedGrams());
    }
}
