package anzaddress

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type nzAddressFixture struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Delivery string `json:"delivery"`
	Locality string `json:"locality"`
	Region   string `json:"region"`
	Postcode string `json:"postcode"`
}

func TestNewZealandAddressFixtures(t *testing.T) {
	raw, err := os.ReadFile("testdata/nz_addresses.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var fixtures []nzAddressFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decode fixtures: %v", err)
	}

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			address, err := Parse(fixture.Input)
			if err != nil {
				t.Fatalf("Parse returned an error: %v", err)
			}
			if !address.IsValid() {
				t.Fatalf("expected valid address, got errors: %v", address.Errors)
			}
			if got, want := address.Country, CountryNZ; got != want {
				t.Errorf("country: got %q, want %q", got, want)
			}
			if got, want := address.FormatDeliveryLine(), fixture.Delivery; got != want {
				t.Errorf("delivery: got %q, want %q", got, want)
			}
			if got, want := address.Locality, fixture.Locality; got != want {
				t.Errorf("locality: got %q, want %q", got, want)
			}
			if got, want := address.State, fixture.Region; got != want {
				t.Errorf("region: got %q, want %q", got, want)
			}
			if got, want := address.Postcode, fixture.Postcode; got != want {
				t.Errorf("postcode: got %q, want %q", got, want)
			}

			reparsed, err := Parse(address.Format())
			if err != nil {
				t.Fatalf("reparse formatted address: %v", err)
			}
			if got, want := reparsed.ComparisonKey(), address.ComparisonKey(); got != want {
				t.Errorf("comparison key after parse-format-parse: got %q, want %q", got, want)
			}
		})
	}
}

func TestParseNewZealandStreetAddress(t *testing.T) {
	address, err := Parse("1 Queen Street, Auckland, Auckland 1010")
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if !address.IsValid() {
		t.Fatalf("expected a valid address, got errors: %v", address.Errors)
	}
	if got, want := address.Country, CountryNZ; got != want {
		t.Errorf("country: got %q, want %q", got, want)
	}
	if got, want := address.Locality, "AUCKLAND"; got != want {
		t.Errorf("locality: got %q, want %q", got, want)
	}
	if got, want := address.State, "AUCKLAND"; got != want {
		t.Errorf("region: got %q, want %q", got, want)
	}
	if got, want := address.Postcode, "1010"; got != want {
		t.Errorf("postcode: got %q, want %q", got, want)
	}
}

func TestNewZealandParserDoesNotStealAustralianFullStateName(t *testing.T) {
	address, err := Parse("1 Collins Street, Melbourne Victoria 3000")
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if got, want := address.Country, CountryAU; got != want {
		t.Errorf("country: got %q, want %q", got, want)
	}
	if got, want := address.State, "VIC"; got != want {
		t.Errorf("state: got %q, want %q", got, want)
	}
}

func TestNewZealandParserRejectsUnknownLocality(t *testing.T) {
	address, err := Parse("1 Queen Street, Not A Locality, Auckland 1010, New Zealand")
	if err != nil {
		t.Fatalf("lenient Parse returned an error: %v", err)
	}
	if address.IsValid() {
		t.Fatalf("expected malformed locality to be invalid: %#v", address)
	}
}

func TestComparisonKeyQualifiesNewZealandAndRejectsCrossCountryMatch(t *testing.T) {
	newZealand := mustParseAddress(t, "1 Queen Street, Auckland, Auckland 1010")
	australia := mustParseAddress(t, "1 Queen Street, Auckland QLD 4034")

	if !strings.HasPrefix(newZealand.ComparisonKey(), "NZ|") {
		t.Fatalf("NZ comparison key is not country-qualified: %q", newZealand.ComparisonKey())
	}
	if newZealand.ComparisonKey() == australia.ComparisonKey() {
		t.Fatal("AU and NZ comparison keys collided")
	}
	if match := CompareAddresses(newZealand, australia); match.Kind != NoMatch {
		t.Fatalf("expected cross-country no match, got %#v", match)
	}
}
