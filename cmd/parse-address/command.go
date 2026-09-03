package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ivanvanderbyl/anzaddress"
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

type comparisonOutput struct {
	Kind             string   `json:"kind"`
	MatchedThrough   string   `json:"matchedThrough,omitempty"`
	MissingFromLeft  []string `json:"missingFromLeft"`
	MissingFromRight []string `json:"missingFromRight"`
	LeftKey          string   `json:"leftKey"`
	RightKey         string   `json:"rightKey"`
}

type errorOutput struct {
	Error string `json:"error"`
}

type outputOptions struct {
	json bool
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	options := &outputOptions{}
	command := newCommand(stdout, stderr, options)
	if err := command.Run(ctx, args); err != nil {
		writeError(stderr, options.json, err)
		return 1
	}
	return 0
}

func newCommand(stdout, stderr io.Writer, options *outputOptions) *cli.Command {
	return &cli.Command{
		Name:      "parse-address",
		Usage:     "parse and compare Australian addresses",
		ArgsUsage: "ADDRESS_INPUT",
		Writer:    stdout,
		ErrWriter: stderr,
		Flags:     []cli.Flag{jsonFlag(options)},
		Commands:  []*cli.Command{newCompareCommand(stdout, options)},
		Action:    parseAction(stdout),
	}
}

func jsonFlag(options *outputOptions) cli.Flag {
	return &cli.BoolFlag{
		Name:        "json",
		Usage:       "write JSON output",
		Destination: &options.json,
	}
}

func parseAction(stdout io.Writer) cli.ActionFunc {
	return func(_ context.Context, command *cli.Command) error {
		if command.NArg() != 1 {
			return fmt.Errorf("expected one address input")
		}

		raw := strings.ReplaceAll(command.Args().First(), `\n`, "\n")
		addresses, err := parseAllStrict(raw)
		if err != nil {
			return err
		}
		if command.Bool("json") {
			return json.NewEncoder(stdout).Encode(newAddressOutputs(addresses))
		}

		formatted := make([]string, len(addresses))
		for i, address := range addresses {
			formatted[i] = address.Format()
		}
		_, err = fmt.Fprintln(stdout, strings.Join(formatted, "\n\n"))
		return err
	}
}

func newCompareCommand(stdout io.Writer, options *outputOptions) *cli.Command {
	return &cli.Command{
		Name:      "compare",
		Usage:     "compare two Australian addresses",
		ArgsUsage: "ADDRESS_A ADDRESS_B",
		Flags:     []cli.Flag{jsonFlag(options)},
		Action: func(_ context.Context, command *cli.Command) error {
			if command.NArg() != 2 {
				return fmt.Errorf("expected two addresses")
			}

			left, err := parseStrict(command.Args().Get(0))
			if err != nil {
				return fmt.Errorf("left address: %w", err)
			}
			right, err := parseStrict(command.Args().Get(1))
			if err != nil {
				return fmt.Errorf("right address: %w", err)
			}

			match := anzaddress.CompareAddresses(left, right)
			if command.Bool("json") {
				return json.NewEncoder(stdout).Encode(newComparisonOutput(match))
			}
			return writeComparison(stdout, match)
		},
	}
}

func writeError(writer io.Writer, jsonOutput bool, err error) {
	if jsonOutput {
		_ = json.NewEncoder(writer).Encode(errorOutput{Error: err.Error()})
		return
	}
	_, _ = fmt.Fprintln(writer, err)
}

func parseStrict(raw string) (*anzaddress.ParsedAddress, error) {
	return anzaddress.NewParser(anzaddress.WithStrict(true)).Parse(raw)
}

func parseAllStrict(raw string) ([]*anzaddress.ParsedAddress, error) {
	return anzaddress.NewParser(anzaddress.WithStrict(true)).ParseAll(raw)
}

func newAddressOutputs(addresses []*anzaddress.ParsedAddress) []addressOutput {
	outputs := make([]addressOutput, len(addresses))
	for i, address := range addresses {
		outputs[i] = newAddressOutput(address)
	}
	return outputs
}

func newAddressOutput(address *anzaddress.ParsedAddress) addressOutput {
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
		case anzaddress.DeliveryPointStreet:
			delivery.Kind = "street"
			delivery.Unit = point.Street.Unit
			delivery.Level = point.Street.Level
			delivery.StreetNumber = point.Street.StreetNumber
			delivery.StreetName = point.Street.StreetName
			delivery.StreetType = point.Street.StreetType
			delivery.StreetSuffix = point.Street.StreetSuffix
		case anzaddress.DeliveryPointPostal:
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

func newComparisonOutput(match anzaddress.AddressMatch) comparisonOutput {
	return comparisonOutput{
		Kind:             matchKindName(match.Kind),
		MatchedThrough:   matchComponentName(match.MatchedThrough),
		MissingFromLeft:  matchComponentNames(match.MissingFromLeft),
		MissingFromRight: matchComponentNames(match.MissingFromRight),
		LeftKey:          match.LeftKey,
		RightKey:         match.RightKey,
	}
}

func writeComparison(writer io.Writer, match anzaddress.AddressMatch) error {
	if _, err := fmt.Fprintln(writer, matchKindName(match.Kind)); err != nil {
		return err
	}
	if match.Kind != anzaddress.PartialMatch {
		return nil
	}
	if match.MatchedThrough != anzaddress.MatchNone {
		if _, err := fmt.Fprintf(writer, "matched through: %s\n", matchComponentName(match.MatchedThrough)); err != nil {
			return err
		}
	}
	if len(match.MissingFromLeft) > 0 {
		if _, err := fmt.Fprintf(writer, "missing from left: %s\n", strings.Join(matchComponentNames(match.MissingFromLeft), ", ")); err != nil {
			return err
		}
	}
	if len(match.MissingFromRight) > 0 {
		if _, err := fmt.Fprintf(writer, "missing from right: %s\n", strings.Join(matchComponentNames(match.MissingFromRight), ", ")); err != nil {
			return err
		}
	}
	return nil
}

func matchKindName(kind anzaddress.MatchKind) string {
	switch kind {
	case anzaddress.ExactMatch:
		return "exact"
	case anzaddress.PartialMatch:
		return "partial"
	default:
		return "no match"
	}
}

func matchComponentNames(components []anzaddress.MatchComponent) []string {
	names := make([]string, 0, len(components))
	for _, component := range components {
		if name := matchComponentName(component); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func matchComponentName(component anzaddress.MatchComponent) string {
	switch component {
	case anzaddress.MatchDeliveryPoint:
		return "deliveryPoint"
	case anzaddress.MatchUnit:
		return "unit"
	case anzaddress.MatchLevel:
		return "level"
	case anzaddress.MatchStreetNumber:
		return "streetNumber"
	case anzaddress.MatchStreetName:
		return "streetName"
	case anzaddress.MatchStreetType:
		return "streetType"
	case anzaddress.MatchStreetSuffix:
		return "streetSuffix"
	case anzaddress.MatchPostalType:
		return "postalType"
	case anzaddress.MatchPostalNumber:
		return "postalNumber"
	case anzaddress.MatchLocality:
		return "locality"
	case anzaddress.MatchState:
		return "state"
	case anzaddress.MatchPostcode:
		return "postcode"
	default:
		return ""
	}
}
