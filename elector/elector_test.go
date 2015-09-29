package elector

import "testing"

func TestElectorBeginLeaderHandlerExecution(t *testing.T) {
	executed := false
	elector := &Elector{
		BeginLeaderHandler: func() error {
			executed = true
			return nil
		},
	}
	elector.reconcileState(StateLeader)
	if executed == false {
		t.Error("leader handler didn't execute on leader message")
	}
	if elector.state != StateLeader {
		t.Errorf("elector state was not leader after leader message, was %s", elector.state)
	}
}

func TestElectorNotLeaderHandlerExecution(t *testing.T) {
	executed := false

	elector := &Elector{
		EndLeaderHandler: func() error {
			executed = true
			return nil
		},
	}
	elector.state = StateLeader
	elector.reconcileState(StateNotLeader)
	if executed == false {
		t.Error("EndLeaderHandler didnt execute on NOTLEADER message")
	}
	if elector.state != StateNotLeader {
		t.Errorf("elector state was not follower after NOTLEADER message, was %s", elector.state)
	}
}

func TestElectorErrorHandlerExecution(t *testing.T) {
	executed := false

	elector := &Elector{
		ErrorHandler: func() error {
			executed = true
			return nil
		},
	}
	elector.reconcileState(StateError)
	if executed == false {
		t.Error("error handler didnt execute on error message")
	}
	if elector.state != StateNotLeader {
		t.Error("elector state was not NOTLEADER after error message")
	}
}

func TestElectorFailsWithInitializedUpdateChan(t *testing.T) {
	elector := &Elector{
		updates: make(chan State),
	}
	err := elector.Run()
	if err == nil {
		t.Error("elector did not error on existing updates chan")
	}
}
