package handlers

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// CommandHandler executes a command in a shell.
func CommandHandler(command *string) func() error {
	return func() error {
		fmt.Printf("CommandHandler: executing %s.\n", *command)

		cmdStrings := strings.Split(*command, " ")
		fullCmd, err := exec.LookPath(cmdStrings[0])
		if err != nil {
			log.Println("CommandHandler: error finding command %s on path", cmdStrings[0])
			log.Println("CommandHandler: command no on path, unrecoverable")
			log.Fatal(err)
		}

		cmd := exec.Command(fullCmd, cmdStrings[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("CommandHandler: error getting command output")
		} else {
			log.Printf("CommandHandler: output: %s", string(output))
		}
		if err != nil {
			log.Println("CommandHandler: received an error: ", err)
		}
		return err
	}
}

// TimeoutHandler waits the specified timeout before returning.
func TimeoutHandler(timeout *time.Duration) func() error {
	return func() error {
		time.Sleep(*timeout)
		return nil
	}
}
