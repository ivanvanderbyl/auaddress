package anzaddress

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type GNAFTestAddress struct {
	FormattedAddress string `json:"formatted_address"`
	Unit             string `json:"unit,omitempty"`
	Level            string `json:"level,omitempty"`
	StreetNumber     string `json:"street_number,omitempty"`
	StreetName       string `json:"street_name,omitempty"`
	StreetType       string `json:"street_type,omitempty"`
	StreetSuffix     string `json:"street_suffix,omitempty"`
	Locality         string `json:"locality"`
	State            string `json:"state"`
	Postcode         string `json:"postcode"`
	BuildingName     string `json:"building_name,omitempty"`
}

func loadGNAFTestData(t *testing.T) []GNAFTestAddress {
	t.Helper()

	data, err := os.ReadFile("testdata/gnaf_addresses.json")
	if err != nil {
		t.Skipf("G-NAF test data not available: %v", err)
		return nil
	}

	var addresses []GNAFTestAddress
	if err := json.Unmarshal(data, &addresses); err != nil {
		t.Fatalf("Failed to parse G-NAF test data: %v", err)
	}

	return addresses
}

func TestGNAFAddressParsing(t *testing.T) {
	addresses := loadGNAFTestData(t)
	if len(addresses) == 0 {
		t.Skip("No G-NAF test data available")
	}

	t.Logf("Testing %d G-NAF addresses", len(addresses))

	var passed, failed int
	var failures []string

	for i, expected := range addresses {
		parsed, err := Parse(expected.FormattedAddress)
		if err != nil {
			failed++
			if len(failures) < 20 {
				failures = append(failures, formatFailure(i, expected, parsed, err.Error()))
			}
			continue
		}

		if !validateParsedAddress(expected, parsed) {
			failed++
			if len(failures) < 20 {
				failures = append(failures, formatMismatch(i, expected, parsed))
			}
		} else {
			passed++
		}
	}

	t.Logf("Results: %d passed, %d failed (%.1f%% success rate)",
		passed, failed, float64(passed)/float64(len(addresses))*100)

	if len(failures) > 0 {
		t.Logf("\nFirst %d failures:", len(failures))
		for _, f := range failures {
			t.Log(f)
		}
	}

	successRate := float64(passed) / float64(len(addresses)) * 100
	if successRate < 80 {
		t.Errorf("Success rate %.1f%% is below 80%% threshold", successRate)
	}
}

func TestGNAFLocalityParsing(t *testing.T) {
	addresses := loadGNAFTestData(t)
	if len(addresses) == 0 {
		t.Skip("No G-NAF test data available")
	}

	var correct, total int

	for _, expected := range addresses {
		parsed, err := Parse(expected.FormattedAddress)
		if err != nil {
			continue
		}

		total++
		if parsed.Locality == expected.Locality &&
			parsed.State == expected.State &&
			parsed.Postcode == expected.Postcode {
			correct++
		}
	}

	t.Logf("Locality parsing: %d/%d correct (%.1f%%)",
		correct, total, float64(correct)/float64(total)*100)

	if float64(correct)/float64(total) < 0.99 {
		t.Errorf("Locality parsing accuracy below 99%%")
	}
}

func TestGNAFStreetParsing(t *testing.T) {
	addresses := loadGNAFTestData(t)
	if len(addresses) == 0 {
		t.Skip("No G-NAF test data available")
	}

	var numberCorrect, nameCorrect, typeCorrect, total int

	for _, expected := range addresses {
		if expected.StreetNumber == "" {
			continue
		}

		parsed, err := Parse(expected.FormattedAddress)
		if err != nil {
			continue
		}

		total++

		if parsed.StreetNumber == expected.StreetNumber {
			numberCorrect++
		}

		if strings.EqualFold(parsed.StreetName, expected.StreetName) {
			nameCorrect++
		}

		if streetTypeMatches(parsed.StreetType, expected.StreetType) {
			typeCorrect++
		}
	}

	if total > 0 {
		t.Logf("Street number: %d/%d (%.1f%%)", numberCorrect, total, float64(numberCorrect)/float64(total)*100)
		t.Logf("Street name: %d/%d (%.1f%%)", nameCorrect, total, float64(nameCorrect)/float64(total)*100)
		t.Logf("Street type: %d/%d (%.1f%%)", typeCorrect, total, float64(typeCorrect)/float64(total)*100)
	}
}

func TestGNAFUnitParsing(t *testing.T) {
	addresses := loadGNAFTestData(t)
	if len(addresses) == 0 {
		t.Skip("No G-NAF test data available")
	}

	var correct, total int

	for _, expected := range addresses {
		if expected.Unit == "" {
			continue
		}

		parsed, err := Parse(expected.FormattedAddress)
		if err != nil {
			continue
		}

		total++

		if unitMatches(parsed.Unit, expected.Unit) {
			correct++
		}
	}

	if total > 0 {
		t.Logf("Unit parsing: %d/%d correct (%.1f%%)",
			correct, total, float64(correct)/float64(total)*100)
	}
}

func TestGNAFAllStates(t *testing.T) {
	addresses := loadGNAFTestData(t)
	if len(addresses) == 0 {
		t.Skip("No G-NAF test data available")
	}

	stateStats := make(map[string]struct{ passed, failed int })

	for _, expected := range addresses {
		parsed, err := Parse(expected.FormattedAddress)

		stats := stateStats[expected.State]
		if err == nil && parsed.State == expected.State && parsed.Postcode == expected.Postcode {
			stats.passed++
		} else {
			stats.failed++
		}
		stateStats[expected.State] = stats
	}

	for state, stats := range stateStats {
		total := stats.passed + stats.failed
		if total > 0 {
			rate := float64(stats.passed) / float64(total) * 100
			t.Logf("%s: %d/%d (%.1f%%)", state, stats.passed, total, rate)
		}
	}
}

func validateParsedAddress(expected GNAFTestAddress, parsed *ParsedAddress) bool {
	if parsed.Locality != expected.Locality {
		return false
	}
	if parsed.State != expected.State {
		return false
	}
	if parsed.Postcode != expected.Postcode {
		return false
	}
	return true
}

func streetTypeMatches(parsed, expected string) bool {
	if parsed == expected {
		return true
	}

	parsedNorm, ok1 := streetTypes[strings.ToUpper(parsed)]
	expectedNorm, ok2 := streetTypes[strings.ToUpper(expected)]

	if ok1 && ok2 {
		return parsedNorm == expectedNorm
	}

	return strings.EqualFold(parsed, expected)
}

func unitMatches(parsed, expected string) bool {
	parsedNorm := strings.ToUpper(strings.TrimSpace(parsed))
	expectedNorm := strings.ToUpper(strings.TrimSpace(expected))

	if parsedNorm == expectedNorm {
		return true
	}

	parsedParts := strings.Fields(parsedNorm)
	expectedParts := strings.Fields(expectedNorm)

	if len(parsedParts) > 0 && len(expectedParts) > 0 {
		return parsedParts[len(parsedParts)-1] == expectedParts[len(expectedParts)-1]
	}

	return false
}

func formatFailure(idx int, expected GNAFTestAddress, parsed *ParsedAddress, errMsg string) string {
	return "---\n" +
		"Input:\n" + expected.FormattedAddress + "\n" +
		"Error: " + errMsg
}

func formatMismatch(idx int, expected GNAFTestAddress, parsed *ParsedAddress) string {
	var diffs []string

	if parsed.Locality != expected.Locality {
		diffs = append(diffs, "locality: got "+parsed.Locality+", want "+expected.Locality)
	}
	if parsed.State != expected.State {
		diffs = append(diffs, "state: got "+parsed.State+", want "+expected.State)
	}
	if parsed.Postcode != expected.Postcode {
		diffs = append(diffs, "postcode: got "+parsed.Postcode+", want "+expected.Postcode)
	}

	return "---\n" +
		"Input:\n" + expected.FormattedAddress + "\n" +
		"Diffs: " + strings.Join(diffs, "; ")
}

func BenchmarkGNAFParsing(b *testing.B) {
	data, err := os.ReadFile("testdata/gnaf_addresses.json")
	if err != nil {
		b.Skipf("G-NAF test data not available: %v", err)
	}

	var addresses []GNAFTestAddress
	if err := json.Unmarshal(data, &addresses); err != nil {
		b.Fatalf("Failed to parse G-NAF test data: %v", err)
	}

	if len(addresses) == 0 {
		b.Skip("No addresses to benchmark")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		addr := addresses[i%len(addresses)]
		_, _ = Parse(addr.FormattedAddress)
	}
}

func BenchmarkTokenGrammar(b *testing.B) {
	benchmarks := []struct {
		name     string
		input    string
		parseAll bool
	}{
		{
			name:  "partial_single_line",
			input: "123 Main Street, Richmond",
		},
		{
			name:  "split_across_lines",
			input: "123 Main\nStreet\nRichmond\nVIC 3121",
		},
		{
			name:  "mixed_delivery_points",
			input: "123 Main Street, PO Box 42, Richmond VIC 3121",
		},
		{
			name:     "multiple_addresses",
			input:    "Level 8, 259 George Street, Sydney NSW 2000\nGPO Box 33, Sydney NSW 2001",
			parseAll: true,
		},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if benchmark.parseAll {
					_, _ = ParseAll(benchmark.input)
					continue
				}
				_, _ = Parse(benchmark.input)
			}
		})
	}
}
