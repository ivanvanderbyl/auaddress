package main

import (
	"context"
	"fmt"
	"io"

	"github.com/ivanvanderbyl/auaddress"
	cli "github.com/urfave/cli/v3"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	command := newCommand(stdout)
	if err := command.Run(ctx, args); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func newCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:      "parse-address",
		Usage:     "parse and compare Australian addresses",
		ArgsUsage: "ADDRESS",
		Action: func(_ context.Context, command *cli.Command) error {
			if command.NArg() != 1 {
				return fmt.Errorf("expected one address")
			}

			parser := auaddress.NewParser(auaddress.WithStrict(true))
			address, err := parser.Parse(command.Args().First())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, address.Format())
			return err
		},
	}
}
