package domain

import (
	"math/big"
	"time"

	"github.com/google/uuid"
)

// Arena is an ISO-3166 alpha-2 jurisdiction code.
type Arena string

const (
	ArenaTR Arena = "TR"
	ArenaCH Arena = "CH"
	ArenaAE Arena = "AE"
	ArenaEU Arena = "EU" // Liechtenstein/MiCA için şemsiye
)

// MintRequest is the input to the mint saga.
// Corresponds to the on-chain IMintController.MintRequest struct.
type MintRequest struct {
	AllocationID uuid.UUID // == on-chain proposalId (bytes32)
	OrderID      uuid.UUID
	To           Address   // hedef kullanıcı cüzdanı (KYC'li)
	AmountWei    *big.Int  // gram * 1e18
	Arena        Arena
	RequestedAt  time.Time
}

// BurnRequest is the input to the burn/redemption saga.
type BurnRequest struct {
	OrderID        uuid.UUID
	From           Address
	AmountWei      *big.Int
	RedemptionType RedemptionType
	PayoutRef      string // IBAN hash veya fiziksel adres referansı
}

// RedemptionType mirrors IBurnController.RedemptionType.
type RedemptionType uint8

const (
	RedemptionCashBack RedemptionType = 0
	RedemptionPhysical RedemptionType = 1
)

// Address is a 20-byte Ethereum address (same as pkg/chain.Address but duplicated
// here so the domain is stdlib-only and test-friendly).
type Address [20]byte

// GoldBar represents a physical bar in a vault.
type GoldBar struct {
	SerialNo     string
	VaultID      uuid.UUID
	WeightGrams  *big.Int // wei cinsinden (1kg = 1000 * 1e18)
	Purity999    int      // 9999 = %99.99
	RefinerLBMA  string
	Status       BarStatus
	AllocatedSum *big.Int // bu çubuktan tahsis edilmiş toplam (fractional allocation destekli)
}

// BarStatus durum etiketleri.
type BarStatus string

const (
	BarInVault    BarStatus = "in_vault"
	BarAllocated  BarStatus = "allocated"
	BarInTransit  BarStatus = "in_transit"
	BarRedeemed   BarStatus = "redeemed"
	BarBurned     BarStatus = "burned"
)

// BarAllocation bir mint operasyonuna hangi çubukların ne kadar gramı tahsis edildi.
type BarAllocation struct {
	AllocationID     uuid.UUID // sagaya göre primary
	SagaID           uuid.UUID
	BarSerial        string
	AllocatedWei     *big.Int
	AllocatedAt      time.Time
	ReleasedAt       *time.Time // burn/redeem'de dolar
}
