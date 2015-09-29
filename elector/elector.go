package elector

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Elector is a _stateful_ leader elector. It uses etcd to manage leader election based on the configured `Key`, executing `LeaderHandler` when newly elected as leader, and `FollowerHandler` when losing leader state. **Elector is stateful - it can be queried for its state, but this tradeoff means it cannot be 'Run()' multiple times.**
type Elector struct {
	Key                *string
	BeginLeaderHandler Handler
	EndLeaderHandler   Handler
	ErrorHandler       Handler

	ElectionBackend ElectionBackend

	updates chan State
	state   State
}

// Handler is registered and executed on leader election events.
type Handler func() error

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

	// 0 so blocks on updates while error handler is executing
	e.updates = make(chan State)

	go e.reconcileStateLoop()
	go e.ElectionBackend.ElectionLoop(e.updates)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		e.updates <- StateNotLeader
		log.Println("Allowing 10s for StateNotLeader to process.")
		time.Sleep(10 * time.Second)
	}()

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
			err := e.BeginLeaderHandler()
			if err != nil {
				log.Println("state: LeaderHandler returned an error; sending error state.")
				e.updates <- StateError
			} else {
				e.state = StateLeader
			}
		}

	case StateNotLeader:
		if e.state == StateLeader {
			log.Println("state: received NOTLEADER (was LEADER)")
			err := e.EndLeaderHandler()
			if err != nil {
				log.Println("state: EndLeaderHandler returned an error; sending error state.")
				log.Println("state: Not transitioning to StateNotLeader - EndLeaderHandler failed (sending StateError). (will retry)")
				e.updates <- StateError
			} else {
				e.state = StateNotLeader
			}
		} else {
			log.Printf("state: received NOTLEADER (was %s)\n", e.state)
			e.state = StateNotLeader
		}

	case StateError:
		log.Println("state: received ERROR")
		if e.state == StateLeader {
			log.Println("state: received ERROR (was LEADER)")

			retries := 12
			failed := 0

			var timeout time.Duration = 5

			for failed < retries {
				if err := e.EndLeaderHandler(); err != nil {
					log.Println("state: EndLeaderHandler returned an error; already rcvd error message.")
					log.Println("state: Not transitioning to StateNotLeader - will retry in 5s.")
					failed++
					time.Sleep(timeout * time.Second)
				} else {
					e.state = StateNotLeader
					break
				}
			}
			if e.state != StateNotLeader {
				log.Printf("state: FAILURE: failed to execute EndLeaderHandler %d times every %ds.\n", failed, timeout)
				log.Fatal("state: FAILURE: exiting. the state of this system may be inconsistent.")
			}
		}
		if e.state == StateNotLeader {
			e.state = StateError
			err := e.ErrorHandler()
			if err != nil {
				log.Println("state: ErrorHandler returned an error; sending another error state.")
				e.updates <- StateError
			} else {
				log.Println("state: ErrorHandler returned successful; setting state to NotLeader and draining queue.")
				numLost := len(e.updates)
				log.Printf("state: lost %d updates during ErrorHandler execution.", numLost)
				for i := 0; i < numLost; i++ {
					log.Printf("state: lost %s during ErrorHandler execution.", <-e.updates)
				}
				e.state = StateNotLeader
			}
		}
	}
}
