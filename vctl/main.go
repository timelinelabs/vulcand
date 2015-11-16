package main

import (
	"os"

	"github.com/timelinelabs/vulcand/plugin/registry"
	"github.com/timelinelabs/vulcand/vctl/command"
	"github.com/vulcand/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
)

var vulcanUrl string

func main() {
	log.InitWithConfig(log.Config{Name: "console"})

	cmd := command.NewCommand(registry.GetRegistry())
	err := cmd.Run(os.Args)
	if err != nil {
		log.Errorf("error: %s\n", err)
	}
}
