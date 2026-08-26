package auaddress

import (
	"reflect"
	"testing"
)

func TestComparisonKeyUsesCanonicalComponents(t *testing.T) {
	long, err := Parse("123 Main Street, Richmond VIC 3121")
	if err != nil {
		t.Fatalf("parse long street type: %v", err)
	}
	short, err := Parse("123 Main St, Richmond VIC 3121")
	if err != nil {
		t.Fatalf("parse short street type: %v", err)
	}

	if long.ComparisonKey() != short.ComparisonKey() {
		t.Fatalf("expected normalized aliases to have one key:\nlong:  %s\nshort: %s", long.ComparisonKey(), short.ComparisonKey())
	}
	const expected = "STREET{UNIT=;LEVEL=;NUMBER=123;NAME=MAIN;TYPE=ST;SUFFIX=}|LOCALITY=RICHMOND|STATE=VIC|POSTCODE=3121"
	if long.ComparisonKey() != expected {
		t.Fatalf("unexpected comparison key:\nwant: %s\ngot:  %s", expected, long.ComparisonKey())
	}
}

func TestCompareAddressesExact(t *testing.T) {
	left := mustParseAddress(t, "123 Main Street, Richmond VIC 3121")
	right := mustParseAddress(t, "123 Main St, Richmond VIC 3121")

	match := CompareAddresses(left, right)
	if match.Kind != ExactMatch {
		t.Fatalf("expected exact match, got %#v", match)
	}
	if match.MatchedThrough != MatchPostcode {
		t.Fatalf("expected match through postcode, got %v", match.MatchedThrough)
	}
	if match.LeftKey == "" || match.LeftKey != match.RightKey {
		t.Fatalf("expected equal populated keys, got %#v", match)
	}
}

func TestCompareAddressesMissingStateAndPostcode(t *testing.T) {
	left := mustParseAddress(t, "123 Main Street, Richmond")
	right := mustParseAddress(t, "123 Main St, Richmond VIC 3121")

	match := CompareAddresses(left, right)
	if match.Kind != PartialMatch {
		t.Fatalf("expected partial match, got %#v", match)
	}
	if match.MatchedThrough != MatchLocality {
		t.Fatalf("expected match through locality, got %v", match.MatchedThrough)
	}
	assertComponentsEqual(t, "missing from left", []MatchComponent{MatchState, MatchPostcode}, match.MissingFromLeft)
	assertComponentsEqual(t, "missing from right", nil, match.MissingFromRight)
}

func TestCompareAddressesMissingLevel(t *testing.T) {
	left := mustParseAddress(t, "Level 8, 259 George Street, Sydney NSW 2000")
	right := mustParseAddress(t, "259 George Street, Sydney NSW 2000")

	match := CompareAddresses(left, right)
	if match.Kind != PartialMatch {
		t.Fatalf("expected partial match, got %#v", match)
	}
	if match.MatchedThrough != MatchPostcode {
		t.Fatalf("expected furthest equal component to be postcode, got %v", match.MatchedThrough)
	}
	assertComponentsEqual(t, "missing from left", nil, match.MissingFromLeft)
	assertComponentsEqual(t, "missing from right", []MatchComponent{MatchLevel}, match.MissingFromRight)
}

func TestCompareAddressesConflictingComponents(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{"street number", "123 Main Street, Richmond VIC 3121", "125 Main Street, Richmond VIC 3121"},
		{"locality", "123 Main Street, Richmond VIC 3121", "123 Main Street, Sydney NSW 2000"},
		{"delivery kind", "123 Main Street, Richmond VIC 3121", "PO Box 123, Richmond VIC 3121"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := CompareAddresses(mustParseAddress(t, tt.left), mustParseAddress(t, tt.right))
			if match.Kind != NoMatch {
				t.Fatalf("expected no match, got %#v", match)
			}
		})
	}
}

func mustParseAddress(t *testing.T, raw string) *ParsedAddress {
	t.Helper()
	addr, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return addr
}

func assertComponentsEqual(t *testing.T, name string, expected, got []MatchComponent) {
	t.Helper()
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("%s: expected %#v, got %#v", name, expected, got)
	}
}
