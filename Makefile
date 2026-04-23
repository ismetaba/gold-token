.PHONY: build test test-e2e clean deploy-sepolia verify-sepolia upgrade-sepolia upgrade-all-sepolia

# Load .env if present (silently ignore if missing)
-include .env
export

CONTRACTS_DIR := contracts
FORGE         := forge

# ── Build ──────────────────────────────────────────────────────────────────────

build:
	cd $(CONTRACTS_DIR) && $(FORGE) build

test:
	cd $(CONTRACTS_DIR) && $(FORGE) test -vvv

test-ci:
	cd $(CONTRACTS_DIR) && $(FORGE) test --profile ci -vvv

test-e2e:
	@echo "Running end-to-end tests against the Docker Compose stack..."
	./e2e/e2e_test.sh

clean:
	cd $(CONTRACTS_DIR) && $(FORGE) clean

# ── Sepolia deployment ─────────────────────────────────────────────────────────
# Prerequisites:
#   1. Copy .env.example → .env and fill in all values.
#   2. Ensure the deployer wallet has Sepolia ETH for gas.
#   3. mkdir -p contracts/deployments
#
# Usage:
#   make deploy-sepolia          — dry-run (no broadcast)
#   make deploy-sepolia BROADCAST=1  — live broadcast + writes deployments/11155111.json

BROADCAST ?=
BROADCAST_FLAGS := $(if $(BROADCAST),--broadcast --slow,)

deploy-sepolia:
	@mkdir -p $(CONTRACTS_DIR)/deployments
	cd $(CONTRACTS_DIR) && $(FORGE) script script/Deploy.s.sol \
		--rpc-url sepolia \
		$(BROADCAST_FLAGS) \
		-vvvv

# ── Etherscan verification ─────────────────────────────────────────────────────
# Reads deployed addresses from deployments/11155111.json produced by deploy-sepolia.
# Run only after BROADCAST=1 deploy completes.

SEPOLIA_CHAIN_ID := 11155111

verify-sepolia: _check-registry
	@echo "Verifying contracts on Sepolia Etherscan..."
	$(eval GOLD_TOKEN   := $(shell jq -r '.goldToken'           $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json))
	$(eval COMPLIANCE   := $(shell jq -r '.complianceRegistry'  $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json))
	$(eval ORACLE       := $(shell jq -r '.reserveOracle'       $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json))
	$(eval MINT_CTRL    := $(shell jq -r '.mintController'      $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json))
	$(eval BURN_CTRL    := $(shell jq -r '.burnController'      $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json))

	cd $(CONTRACTS_DIR) && $(FORGE) verify-contract $(GOLD_TOKEN)  src/GoldToken.sol:GoldToken \
		--chain sepolia --watch
	cd $(CONTRACTS_DIR) && $(FORGE) verify-contract $(COMPLIANCE)  src/ComplianceRegistry.sol:ComplianceRegistry \
		--chain sepolia --watch
	cd $(CONTRACTS_DIR) && $(FORGE) verify-contract $(ORACLE)      src/ReserveOracle.sol:ReserveOracle \
		--chain sepolia --watch
	cd $(CONTRACTS_DIR) && $(FORGE) verify-contract $(MINT_CTRL)   src/MintController.sol:MintController \
		--chain sepolia --watch
	cd $(CONTRACTS_DIR) && $(FORGE) verify-contract $(BURN_CTRL)   src/BurnController.sol:BurnController \
		--chain sepolia --watch

# ── Sepolia upgrades ───────────────────────────────────────────────────────────
# Upgrades a single contract. BROADCAST=1 to go live.
# Example: make upgrade-sepolia CONTRACT=upgradeGoldToken BROADCAST=1

CONTRACT ?= upgradeAll

upgrade-sepolia:
	cd $(CONTRACTS_DIR) && $(FORGE) script script/Upgrade.s.sol \
		--sig "$(CONTRACT)()" \
		--rpc-url sepolia \
		$(BROADCAST_FLAGS) \
		-vvvv

# Convenience target: upgrade all UUPS proxies in one broadcast.
upgrade-all-sepolia:
	$(MAKE) upgrade-sepolia CONTRACT=upgradeAll

# ── Helpers ────────────────────────────────────────────────────────────────────

_check-registry:
	@test -f $(CONTRACTS_DIR)/deployments/$(SEPOLIA_CHAIN_ID).json \
		|| (echo "ERROR: deployments/$(SEPOLIA_CHAIN_ID).json not found. Run 'make deploy-sepolia BROADCAST=1' first." && exit 1)
