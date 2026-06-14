package domain

import "testing"

func TestMintStateTransitionAllowsHappyPath(t *testing.T) {
	path := []SagaState{
		StateCreated, StateReservingBars, StateProposing,
		StateAwaitingApprovals, StateExecuting, StateCompleted,
	}
	for i := 0; i+1 < len(path); i++ {
		if err := MintStateTransition(path[i], path[i+1]); err != nil {
			t.Fatalf("happy-path transition %s→%s rejected: %v", path[i], path[i+1], err)
		}
	}
}

func TestMintStateTransitionAllowsFailureBranches(t *testing.T) {
	ok := [][2]SagaState{
		{StateReservingBars, StateFailedNoStock},
		{StateAwaitingApprovals, StateFailedApprovalTimeout},
		{StateExecuting, StateFailedReserveInvariant},
		{StateCreated, StateFailed},
	}
	for _, tc := range ok {
		if err := MintStateTransition(tc[0], tc[1]); err != nil {
			t.Errorf("expected %s→%s allowed: %v", tc[0], tc[1], err)
		}
	}
}

func TestMintStateTransitionRejectsIllegal(t *testing.T) {
	bad := [][2]SagaState{
		{StateCreated, StateExecuting},          // skips steps
		{StateCompleted, StateProposing},        // terminal cannot move
		{StateExecuting, StateReservingBars},    // no going back
		{StateAwaitingApprovals, StateCompleted}, // must execute first
	}
	for _, tc := range bad {
		if err := MintStateTransition(tc[0], tc[1]); err == nil {
			t.Errorf("expected %s→%s rejected", tc[0], tc[1])
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []SagaState{
		StateCompleted, StateBurnExecuted, StateFailed,
		StateFailedNoStock, StateFailedApprovalTimeout, StateFailedReserveInvariant,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	nonTerminal := []SagaState{
		StateCreated, StateReservingBars, StateProposing,
		StateAwaitingApprovals, StateExecuting,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}
