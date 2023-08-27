package main

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "Guardian"
	app.Usage = "Guardian for blockchain"
	app.Compiled = time.Now()

	cli.VersionPrinter = func(c *cli.Context) {
		printVersion()
	}

	// global flags
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "repo",
			Usage: "Guardian storage repo path",
		},
	}

	app.Commands = []*cli.Command{
		configCMD,
		{
			Name:   "start",
			Usage:  "Start a long-running daemon process",
			Action: start,
		},
		{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "Guardian version",
			Action: func(ctx *cli.Context) error {
				printVersion()
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
