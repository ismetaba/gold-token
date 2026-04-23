//go:build !cgo

package signer

import "fmt"

func newSoftHSMSigner(_ Config) (Signer, error) {
	return nil, fmt.Errorf("signer: softhsm requires CGO (build with CGO_ENABLED=1)")
}
