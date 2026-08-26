package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ivanvanderbyl/auaddress"
	cli "github.com/urfave/cli/v3"
)

type addressOutput struct {
	DeliveryPoints []deliveryPointOutput `json:"deliveryPoints"`
	NameLines      []string              `json:"nameLines,omitempty"`
	Locality       string                `json:"locality"`
	State          string                `json:"state,omitempty"`
	Postcode       string                `json:"postcode,omitempty"`
}

type deliveryPointOutput struct {
	Kind         string `json:"kind"`
	Unit         string `json:"unit,omitempty"`
	Level        string `json:"level,omitempty"`
	StreetNumber string `json:"streetNumber,omitempty"`
	StreetName   string `json:"streetName,omitempty"`
	StreetType   string `json:"streetType,omitempty"`
	StreetSuffix string `json:"streetSuffix,omitempty"`
	PostalType   string `json:"postalType,omitempty"`
	PostalNumber string `json:"postalNumber,omitempty"`
}

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
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "write JSON output",
			},
		},
		Action: func(_ context.Context, command *cli.Command) error {
			if command.NArg() != 1 {
				return fmt.Errorf("expected one address")
			}

			parser := auaddress.NewParser(auaddress.WithStrict(true))
			address, err := parser.Parse(command.Args().First())
			if err != nil {
				return err
			}
			if command.Bool("json") {
				return json.NewEncoder(stdout).Encode(newAddressOutput(address))
			}
			_, err = fmt.Fprintln(stdout, address.Format())
			return err
		},
	}
}

func newAddressOutput(address *auaddress.ParsedAddress) addressOutput {
	output := addressOutput{
		DeliveryPoints: make([]deliveryPointOutput, 0, len(address.DeliveryPoints)),
		NameLines:      append([]string(nil), address.NameLines...),
		Locality:       address.Locality,
		State:          address.State,
		Postcode:       address.Postcode,
	}
	for _, point := range address.DeliveryPoints {
		delivery := deliveryPointOutput{}
		switch point.Kind {
		case auaddress.DeliveryPointStreet:
			delivery.Kind = "street"
			delivery.Unit = point.Street.Unit
			delivery.Level = point.Street.Level
			delivery.StreetNumber = point.Street.StreetNumber
			delivery.StreetName = point.Street.StreetName
			delivery.StreetType = point.Street.StreetType
			delivery.StreetSuffix = point.Street.StreetSuffix
		case auaddress.DeliveryPointPostal:
			delivery.Kind = "postal"
			delivery.PostalType = point.Postal.Type
			delivery.PostalNumber = point.Postal.Number
		default:
			continue
		}
		output.DeliveryPoints = append(output.DeliveryPoints, delivery)
	}
	return output
}
