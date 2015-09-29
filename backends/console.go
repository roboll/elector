package backends

import (
	"fmt"
	"log"
	"time"

	"github.com/roboll/elector/elector"
)

// Console backend is an election backend powered by commands from the console.
// It is used for testing only.
type Console struct{}

// ElectionLoop listens on the console for user input and translates that to state changes.
func (c *Console) ElectionLoop(updates chan elector.State) error {
	log.Println("Starting election loop.")

	for {
		var next string
		time.Sleep(1 * time.Second)
		fmt.Print("Enter next state: ")
		_, err := fmt.Scanln(&next)
		if err != nil {
			log.Println("Error while trying to get user input.")
			log.Println(err.Error())
			continue
		}

		switch next {
		case "LEADER":
			updates <- elector.StateLeader
		case "NOTLEADER":
			updates <- elector.StateNotLeader
		case "ERROR":
			updates <- elector.StateError
		default:
			log.Fatal("unexpected user input")
		}
	}
	return nil
}
