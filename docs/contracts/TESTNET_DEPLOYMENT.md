# GOLD Token — Sepolia Testnet Deployment

**Deployed:** 2026-04-24  
**Network:** Ethereum Sepolia (chain ID 11155111)  
**Deployer:** `0x02445B87D4B0CC9CE52955CbBea1B00160633b83`

---

## Contract Addresses

| Contract | Proxy Address | Etherscan |
|---|---|---|
| GoldToken | `0x349bdDC43cdba65F666F52ec23C1693FC510dB06` | [view](https://sepolia.etherscan.io/address/0x349bdDC43cdba65F666F52ec23C1693FC510dB06) |
| ComplianceRegistry | `0x7b2dE6dCfc3B90bF8678D44c49D94Bda8293efF0` | [view](https://sepolia.etherscan.io/address/0x7b2dE6dCfc3B90bF8678D44c49D94Bda8293efF0) |
| MintController | `0x8C7c3B0CF8f89E53E24714aEf1394731151240Dd` | [view](https://sepolia.etherscan.io/address/0x8C7c3B0CF8f89E53E24714aEf1394731151240Dd) |
| BurnController | `0x96F5887418bA664f6992C1ebe5b6ba1ac7AC9Ae0` | [view](https://sepolia.etherscan.io/address/0x96F5887418bA664f6992C1ebe5b6ba1ac7AC9Ae0) |
| ReserveOracle | `0x8D79aD1AA9De94e83972389F151969190E87ED09` | [view](https://sepolia.etherscan.io/address/0x8D79aD1AA9De94e83972389F151969190E87ED09) |

Implementation addresses (behind proxies):

| Contract | Implementation Address |
|---|---|
| ComplianceRegistry impl | `0x2a204a3b207ca0af928708a2eba5826eeeacf874` |
| GoldToken impl | `0xe9568afd7d1d3a43119b20dfcab4fff816c064ad` |
| MintController impl | `0x0eb0311124db06bf5dd4194bda3dd41958726030` |
| BurnController impl | `0xedc245ef64f19f5a0440d866f06ccbad73c11048` |

Machine-readable address registry: [`contracts/deployments/11155111.json`](../../contracts/deployments/11155111.json)

---

## Deployment Process

```bash
cd contracts
source ../.env

forge script script/Deploy.s.sol \
  --rpc-url "$SEPOLIA_RPC_URL" \
  --broadcast \
  --slow \
  --verify \
  --etherscan-api-key "$ETHERSCAN_API_KEY"
```

The script:
1. Deploys 5 implementation contracts
2. Wraps 4 of them in ERC1967 (UUPS) proxies
3. Links controllers to GoldToken (`setMintController`, `setBurnController`)
4. Writes `contracts/deployments/11155111.json`

All 11 transactions succeeded (status `0x1`). Broadcast logs: `contracts/broadcast/Deploy.s.sol/11155111/run-latest.json`

---

## Etherscan Verification

All implementations submitted for verification on 2026-04-24 using:

```bash
forge verify-contract --chain-id 11155111 \
  --etherscan-api-key "$ETHERSCAN_API_KEY" \
  --rpc-url "$SEPOLIA_RPC_URL" \
  <address> <Contract>
```

ReserveOracle was already verified. Proxies are verified via the implementation ABI.

---

## On-chain Smoke Tests — PASS

Executed 2026-04-24 with `cast call --rpc-url $SEPOLIA_RPC_URL`:

| Check | Expected | Result |
|---|---|---|
| `GoldToken.name()` | "GOLD Gold" | ✅ |
| `GoldToken.symbol()` | "GOLD" | ✅ |
| `GoldToken.decimals()` | 18 | ✅ |
| `GoldToken.totalSupply()` | 0 (fresh) | ✅ |
| `GoldToken.paused()` | false | ✅ |
| `GoldToken.mintController()` | MintController proxy | ✅ |
| `GoldToken.burnController()` | BurnController proxy | ✅ |
| `GoldToken.complianceRegistry()` | ComplianceRegistry proxy | ✅ |
| `ComplianceRegistry.travelRuleThreshold()` | 1000 GOLD (1e21 wei) | ✅ |
| `MintController.approvalThreshold()` | 3 | ✅ |
| `MintController.totalApprovers()` | 5 | ✅ |
| `BurnController.minPhysicalGrams()` | 1000 grams (1e21 wei) | ✅ |
| `ReserveOracle.attestationCount()` | 0 (no attestations yet) | ✅ |
| `ReserveOracle.isAuthorizedAuditor(deployer)` | true | ✅ |

---

## Backend Configuration

Add to your `.env` (already set):

```env
GOLD_TOKEN_PROXY=0x349bdDC43cdba65F666F52ec23C1693FC510dB06
COMPLIANCE_REGISTRY_PROXY=0x7b2dE6dCfc3B90bF8678D44c49D94Bda8293efF0
MINT_CONTROLLER_PROXY=0x8C7c3B0CF8f89E53E24714aEf1394731151240Dd
BURN_CONTROLLER_PROXY=0x96F5887418bA664f6992C1ebe5b6ba1ac7AC9Ae0
CHAIN_RPC_URL=https://ethereum-sepolia-rpc.publicnode.com
CHAIN_ID=31337
```

The backend reads contract addresses from `contracts/deployments/11155111.json` at startup (or from env vars above).

---

## Next Steps

- [ ] **Publish first ReserveOracle attestation** — auditor must sign and call `oracle.publish()` before any mint is possible
- [ ] **KYC-register deployer wallet** — call `ComplianceRegistry.setProfile()` to allow test transactions
- [ ] **Mint test tokens** — propose → 3/5 approve → execute a small mint for smoke testing token transfer
- [ ] **Full e2e stack test** — `./e2e/e2e_test.sh` against docker-compose stack pointed at Sepolia
- [ ] **Mainnet checklist** — replace EOA deployer with Gnosis Safe 3/5, add 7-day TimelockController for upgrades

---

## Notes

- All testnet roles point to the deployer address (`0x02445B87D4B0CC9CE52955CbBea1B00160633b83`) for simplicity
- On mainnet, Treasury must be a Gnosis Safe 3/5 multisig (see `docs/contracts/README.md` §8)
- ReserveOracle is **immutable** by design — no proxy, no upgrade path
