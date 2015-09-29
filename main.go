package main

import (
	"log"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/roboll/elector/backends"
	"github.com/roboll/elector/elector"
	"github.com/roboll/elector/handlers"
)

var release string

func main() {
	app := cli.NewApp()

	app.Name = "elector"
	app.Usage = "elect a leader"

	if release != "" {
		app.Version = release
	} else {
		app.Version = "HEAD"
	}

	app.Flags = flags
	app.Commands = commands

	app.Run(os.Args)
}

var flags = []cli.Flag{
	cli.StringFlag{
		Name:   "keyspace",
		Usage:  "keyspace to elect on",
		EnvVar: "ELECTOR_KEY",
	},
	cli.StringFlag{
		Name:   "backend",
		Usage:  "backend name",
		EnvVar: "ELECTOR_BACKEND",
	},
	cli.StringFlag{
		Name:   "leader-start-command",
		Usage:  "leader start command - runs when state changes to LEADER",
		EnvVar: "ELECTOR_START_COMMAND",
	},
	cli.StringFlag{
		Name:   "leader-end-command",
		Usage:  "leader end command - runs when state changes from LEADER",
		EnvVar: "ELECTOR_STOP_COMMAND",
	},
	cli.StringSliceFlag{
		Name:   "etcd-members",
		Usage:  "etcd members",
		EnvVar: "ELECTOR_ETCD_MEMBERS",
	},
	cli.StringFlag{
		Name:   "instance-id",
		Usage:  "unique id, falls back to hostname",
		EnvVar: "ELECTOR_INSTANCE_ID",
	},
}

var runFlags = []cli.Flag{
	cli.DurationFlag{
		Name:   "error-timeout",
		Usage:  "error timeout - time to wait after an error before resuming candidacy",
		EnvVar: "ELECTOR_ERROR_TIMEOUT",
	},
}

var commands = []cli.Command{
	{
		Name:        "run",
		Description: "begin monitoring key for election",
		Action:      runWithArgs,
		Flags:       runFlags,
	},
}

func runWithArgs(c *cli.Context) {
	timeout := c.Duration("error-timeout")
	if timeout == 0 {
		log.Println("timeout not specified(or specied as 0); using default 30s")
		timeout = 30 * time.Second
	}

	startCmd := c.GlobalString("leader-start-command")
	if startCmd == "" {
		log.Fatal("leader-start-command is required.")
	}

	endCmd := c.GlobalString("leader-end-command")
	if endCmd == "" {
		log.Fatal("leader-end-command is required.")
	}

	backendName := c.GlobalString("backend")
	var backend elector.ElectionBackend

	switch backendName {
	case "etcd":
		keyspace := c.GlobalString("keyspace")
		if keyspace == "" {
			log.Fatal("keyspace is required.")
		}

		members := c.GlobalStringSlice("etcd-members")
		if len(members) < 1 {
			log.Fatal("at least one etcd-members is required.")
		}

		instanceID := c.GlobalString("instance-id")

		caFile := c.GlobalString("ca-file")
		certFile := c.GlobalString("cert-file")
		keyFile := c.GlobalString("key-file")

		backend = &backends.Etcd{
			Members:    &members,
			Keyspace:   &keyspace,
			InstanceID: &instanceID,

			CAFile:   &caFile,
			CertFile: &certFile,
			KeyFile:  &keyFile,
		}
	case "console":
		backend = &backends.Console{}
	default:
		log.Fatal("must specify a valid backend.")
	}

	elector := &elector.Elector{
		StartLeaderHandler: handlers.CommandHandler(&startCmd),
		EndLeaderHandler:   handlers.CommandHandler(&endCmd),
		ErrorHandler:       handlers.TimeoutHandler(&timeout),

		ElectionBackend: backend,
	}

	err := elector.Run()
	if err != nil {
		log.Fatal(err)
	}
}
