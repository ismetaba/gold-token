package events

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/domain"
)

var (
	ErrInvalidAddress = errors.New("events: invalid ethereum address")
	ErrInvalidAmount  = errors.New("events: invalid amount")
)

// parseAddress, "0x..." hex string'i 20-byte Address'e çevirir.
func parseAddress(s string) (domain.Address, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 40 {
		return domain.Address{}, ErrInvalidAddress
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return domain.Address{}, ErrInvalidAddress
	}
	var out domain.Address
	copy(out[:], raw)
	return out, nil
}
