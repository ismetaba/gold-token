package chain

import "errors"

var (
	ErrProposalNotFound      = errors.New("chain: proposal not found")
	ErrInsufficientApprovals = errors.New("chain: insufficient approvals")
	ErrReserveInvariant      = errors.New("chain: reserve invariant violated")
	ErrTxReverted            = errors.New("chain: tx reverted")
)
