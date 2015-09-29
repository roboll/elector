package elector

import (
	"fmt"
	"log"
	"sync"
)

// Elector is a _stateful_ leader elector. It uses etcd to manage leader election based on the configured `Key`, executing `LeaderHandler` when newly elected as leader, and `FollowerHandler` when losing leader state. **Elector is stateful - it can be queried for its state, but this tradeoff means it cannot be 'Run()' multiple times.**
type Elector struct {
	Key                *string
	StartLeaderHandler func() error
	EndLeaderHandler   func() error
	ErrorHandler       func() error

	ElectionBackend ElectionBackend

	updates chan State
	state   State
}

// ElectionBackend handles election, sending state messages as appropriate.
// ElectionBackend executes on the main goroutine, and so is expected not to return.
type ElectionBackend interface {
	//ElectionLoop retrieves state changes from the backend and sends them over the update channel to be handled.
	//Error should only be returned in case of non-recoverable error. For recoverable error use message StateError.
	ElectionLoop(chan State) error
}

// State is a possible state of an elector.
type State string

// Possible elector states enum.
const (
	// StateNotLeader should be used by backends during startup before election has begun, and when not holding leader status.
	// It is also used after the error handler returns successfully but before a new state has been acted upon.
	StateNotLeader State = "NOTLEADER"
	// StateLeader indicates leader state.
	StateLeader State = "LEADER"
	// StateError indicates an error during handler execution.
	// The state reconciliation loop will not respond to state changes until ErrorHandler() returns.
	// When ErrorHandler() returns successful, the state reconciliation loop will drain the update queue and wait for the next update.
	StateError State = "ERROR"
)

// Run starts an elector loop.
func (e *Elector) Run() error {
	log.Println("Starting elector.")
	if e.updates != nil {
		return fmt.Errorf("Elector has already been initialized. Electors can be started once.")
	}

	//0 so blocks on updates while error handler is executing
	e.updates = make(chan State)

	go e.reconcileStateLoop()
	go e.ElectionBackend.ElectionLoop(e.updates)

	group := sync.WaitGroup{}
	group.Add(2)
	defer group.Done()
	group.Wait()
	return nil
}

func (e *Elector) reconcileStateLoop() {
	log.Println("Starting state reconciliation loop.")
	for update := range e.updates {
		e.reconcileState(update)
	}
}

func (e *Elector) reconcileState(update State) {
	switch update {
	case StateLeader:

		if e.state == StateLeader {
			log.Println("state: received LEADER (was already LEADER)")
		} else {
			e.state = StateLeader
			err := e.StartLeaderHandler()
			if err != nil {
				log.Println("state: LeaderHandler returned an error; sending error state.")
				e.updates <- StateError
			}
		}

	case StateNotLeader:
		if e.state == StateLeader {
			log.Println("state: received NOTLEADER (was LEADER)")
			e.state = StateNotLeader
			err := e.EndLeaderHandler()
			if err != nil {
				log.Println("state: EndLeaderHandler returned an error; sending error state.")
				e.updates <- StateError
			}
		} else {
			log.Printf("state: received NOTLEADER (was %s)\n", e.state)
			e.state = StateNotLeader
		}

	case StateError:
		log.Println("state: received ERROR")
		if e.state == StateLeader {
			log.Println("state: received ERROR (was LEADER)")
			err := e.EndLeaderHandler()
			if err != nil {
				log.Println("state: EndLeaderHandler returned an error; already in error state.")
			}
		}
		e.state = StateError
		err := e.ErrorHandler()
		if err != nil {
			log.Println("state: ErrorHandler returned an error; sending another error state.")
			e.updates <- StateError
		} else {
			log.Println("state: ErrorHandler returned successful; setting state to NotLeader and draining queue")
			numLost := len(e.updates)
			log.Printf("state: lost %d updates during ErrorHandler execution.", numLost)
			for i := 0; i < numLost; i++ {
				log.Printf("state: lost %s during ErrorHandler execution.", <-e.updates)
			}
			e.state = StateNotLeader
		}
	}
}
