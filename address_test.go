package auaddress

import (
	"testing"
)

func TestParseSimpleStreetAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParsedAddress
	}{
		{
			name: "simple residential address",
			input: `John Smith
123 Main Street
SYDNEY NSW 2000`,
			expected: ParsedAddress{
				NameLines:    []string{"John Smith"},
				StreetNumber: "123",
				StreetName:   "MAIN",
				StreetType:   "ST",
				Locality:     "SYDNEY",
				State:        "NSW",
				Postcode:     "2000",
			},
		},
		{
			name: "address with unit slash notation",
			input: `Jane Doe
3/45 High Street
MELBOURNE VIC 3000`,
			expected: ParsedAddress{
				NameLines:    []string{"Jane Doe"},
				Unit:         "3",
				StreetNumber: "45",
				StreetName:   "HIGH",
				StreetType:   "ST",
				Locality:     "MELBOURNE",
				State:        "VIC",
				Postcode:     "3000",
			},
		},
		{
			name: "address with unit prefix",
			input: `Bob Builder
Unit 5, 100 George Street
BRISBANE QLD 4000`,
			expected: ParsedAddress{
				NameLines:    []string{"Bob Builder"},
				Unit:         "UNIT 5",
				StreetNumber: "100",
				StreetName:   "GEORGE",
				StreetType:   "ST",
				Locality:     "BRISBANE",
				State:        "QLD",
				Postcode:     "4000",
			},
		},
		{
			name: "address with level",
			input: `Acme Corp
Level 10, 200 Collins Street
MELBOURNE VIC 3000`,
			expected: ParsedAddress{
				NameLines:    []string{"Acme Corp"},
				Level:        "L 10",
				StreetNumber: "200",
				StreetName:   "COLLINS",
				StreetType:   "ST",
				Locality:     "MELBOURNE",
				State:        "VIC",
				Postcode:     "3000",
			},
		},
		{
			name: "address with street suffix direction",
			input: `Alice Wonder
42 King Street North
PERTH WA 6000`,
			expected: ParsedAddress{
				NameLines:    []string{"Alice Wonder"},
				StreetNumber: "42",
				StreetName:   "KING",
				StreetType:   "ST",
				StreetSuffix: "N",
				Locality:     "PERTH",
				State:        "WA",
				Postcode:     "6000",
			},
		},
		{
			name: "address with street number range",
			input: `Big Business
10-12 Market Road
ADELAIDE SA 5000`,
			expected: ParsedAddress{
				NameLines:    []string{"Big Business"},
				StreetNumber: "10-12",
				StreetName:   "MARKET",
				StreetType:   "RD",
				Locality:     "ADELAIDE",
				State:        "SA",
				Postcode:     "5000",
			},
		},
		{
			name: "multi-word locality",
			input: `Rural Person
1 Farm Lane
ALICE SPRINGS NT 0870`,
			expected: ParsedAddress{
				NameLines:    []string{"Rural Person"},
				StreetNumber: "1",
				StreetName:   "FARM",
				StreetType:   "LANE",
				Locality:     "ALICE SPRINGS",
				State:        "NT",
				Postcode:     "0870",
			},
		},
		{
			name: "Tasmania address",
			input: `Island Resident
99 Salamanca Place
HOBART TAS 7000`,
			expected: ParsedAddress{
				NameLines:    []string{"Island Resident"},
				StreetNumber: "99",
				StreetName:   "SALAMANCA",
				StreetType:   "PL",
				Locality:     "HOBART",
				State:        "TAS",
				Postcode:     "7000",
			},
		},
		{
			name: "ACT address",
			input: `Government Worker
1 Parliament Drive
CANBERRA ACT 2600`,
			expected: ParsedAddress{
				NameLines:    []string{"Government Worker"},
				StreetNumber: "1",
				StreetName:   "PARLIAMENT",
				StreetType:   "DR",
				Locality:     "CANBERRA",
				State:        "ACT",
				Postcode:     "2600",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			assertStringSliceEqual(t, "NameLines", tt.expected.NameLines, addr.NameLines)
			assertEqual(t, "Unit", tt.expected.Unit, addr.Unit)
			assertEqual(t, "Level", tt.expected.Level, addr.Level)
			assertEqual(t, "StreetNumber", tt.expected.StreetNumber, addr.StreetNumber)
			assertEqual(t, "StreetName", tt.expected.StreetName, addr.StreetName)
			assertEqual(t, "StreetType", tt.expected.StreetType, addr.StreetType)
			assertEqual(t, "StreetSuffix", tt.expected.StreetSuffix, addr.StreetSuffix)
			assertEqual(t, "Locality", tt.expected.Locality, addr.Locality)
			assertEqual(t, "State", tt.expected.State, addr.State)
			assertEqual(t, "Postcode", tt.expected.Postcode, addr.Postcode)
		})
	}
}

func TestParsePOBoxAddresses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParsedAddress
	}{
		{
			name: "PO Box",
			input: `Company Name
PO Box 1234
SYDNEY NSW 2000`,
			expected: ParsedAddress{
				NameLines:   []string{"Company Name"},
				IsPoBox:     true,
				PoBoxType:   "PO BOX",
				PoBoxNumber: "1234",
				Locality:    "SYDNEY",
				State:       "NSW",
				Postcode:    "2000",
			},
		},
		{
			name: "GPO Box",
			input: `Big Bank
GPO Box 9999
MELBOURNE VIC 3001`,
			expected: ParsedAddress{
				NameLines:   []string{"Big Bank"},
				IsPoBox:     true,
				PoBoxType:   "GPO BOX",
				PoBoxNumber: "9999",
				Locality:    "MELBOURNE",
				State:       "VIC",
				Postcode:    "3001",
			},
		},
		{
			name: "Locked Bag",
			input: `Government Dept
Locked Bag 5000
PARRAMATTA NSW 2124`,
			expected: ParsedAddress{
				NameLines:   []string{"Government Dept"},
				IsPoBox:     true,
				PoBoxType:   "LOCKED BAG",
				PoBoxNumber: "5000",
				Locality:    "PARRAMATTA",
				State:       "NSW",
				Postcode:    "2124",
			},
		},
		{
			name: "Private Bag",
			input: `University
Private Bag 100
CARLTON VIC 3053`,
			expected: ParsedAddress{
				NameLines:   []string{"University"},
				IsPoBox:     true,
				PoBoxType:   "PRIVATE BAG",
				PoBoxNumber: "100",
				Locality:    "CARLTON",
				State:       "VIC",
				Postcode:    "3053",
			},
		},
		{
			name: "Reply Paid",
			input: `Magazine Subscription
Reply Paid 12345
SYDNEY NSW 2000`,
			expected: ParsedAddress{
				NameLines:   []string{"Magazine Subscription"},
				IsPoBox:     true,
				PoBoxType:   "REPLY PAID",
				PoBoxNumber: "12345",
				Locality:    "SYDNEY",
				State:       "NSW",
				Postcode:    "2000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			assertBool(t, "IsPoBox", tt.expected.IsPoBox, addr.IsPoBox)
			assertEqual(t, "PoBoxType", tt.expected.PoBoxType, addr.PoBoxType)
			assertEqual(t, "PoBoxNumber", tt.expected.PoBoxNumber, addr.PoBoxNumber)
			assertEqual(t, "Locality", tt.expected.Locality, addr.Locality)
			assertEqual(t, "State", tt.expected.State, addr.State)
			assertEqual(t, "Postcode", tt.expected.Postcode, addr.Postcode)
		})
	}
}

func TestParseNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Windows line endings",
			input: "John Smith\r\n123 Main Street\r\nSYDNEY NSW 2000",
		},
		{
			name:  "Mixed line endings",
			input: "John Smith\r123 Main Street\nSYDNEY NSW 2000",
		},
		{
			name:  "Extra whitespace",
			input: "  John Smith  \n  123   Main   Street  \n  SYDNEY   NSW   2000  ",
		},
		{
			name:  "Trailing punctuation",
			input: "John Smith,\n123 Main Street.\nSYDNEY NSW 2000,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			assertEqual(t, "Locality", "SYDNEY", addr.Locality)
			assertEqual(t, "State", "NSW", addr.State)
			assertEqual(t, "Postcode", "2000", addr.Postcode)
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr error
	}{
		{
			name:        "empty string",
			input:       "",
			expectedErr: ErrEmptyAddress,
		},
		{
			name:        "whitespace only",
			input:       "   \n  \n  ",
			expectedErr: ErrEmptyAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestStrictMode(t *testing.T) {
	parser := NewParser(WithStrict(true))

	t.Run("invalid postcode in strict mode", func(t *testing.T) {
		_, err := parser.Parse("John Smith\n123 Main Street\nSYDNEY NSW ABCD")
		if err != ErrNoPostcode {
			t.Errorf("expected ErrNoPostcode, got %v", err)
		}
	})

	t.Run("invalid state in strict mode", func(t *testing.T) {
		_, err := parser.Parse("John Smith\n123 Main Street\nSYDNEY XYZ 2000")
		if err != ErrNoState {
			t.Errorf("expected ErrNoState, got %v", err)
		}
	})

	t.Run("missing delivery line in strict mode", func(t *testing.T) {
		_, err := parser.Parse("SYDNEY NSW 2000")
		if err != ErrNoDeliveryLine {
			t.Errorf("expected ErrNoDeliveryLine, got %v", err)
		}
	})
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple address",
			input: `John Smith
123 Main Street
Sydney NSW 2000`,
			expected: `JOHN SMITH
123 MAIN ST
SYDNEY NSW 2000`,
		},
		{
			name: "unit address",
			input: `Jane Doe
3/45 High St
Melbourne VIC 3000`,
			expected: `JANE DOE
3 45 HIGH ST
MELBOURNE VIC 3000`,
		},
		{
			name: "PO Box",
			input: `Company Name
PO Box 1234
Sydney NSW 2000`,
			expected: `COMPANY NAME
PO BOX 1234
SYDNEY NSW 2000`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			formatted := addr.Format()
			if formatted != tt.expected {
				t.Errorf("format mismatch:\nexpected:\n%s\n\ngot:\n%s", tt.expected, formatted)
			}
		})
	}
}

func TestFormatMixedDeliveryPoints(t *testing.T) {
	addr := &ParsedAddress{
		DeliveryPoints: []DeliveryPoint{
			{
				Kind: DeliveryPointStreet,
				Street: StreetDelivery{
					StreetNumber: "123",
					StreetName:   "MAIN",
					StreetType:   "ST",
				},
			},
			{
				Kind: DeliveryPointPostal,
				Postal: PostalDelivery{
					Type:   "PO BOX",
					Number: "42",
				},
			},
		},
		Locality: "RICHMOND",
	}

	assertStringSliceEqual(t, "FormatDeliveryLines", []string{
		"123 MAIN ST",
		"PO BOX 42",
	}, addr.FormatDeliveryLines())
	assertEqual(t, "FormatDeliveryLine", "123 MAIN ST", addr.FormatDeliveryLine())
	assertEqual(t, "Format", "123 MAIN ST\nPO BOX 42\nRICHMOND", addr.Format())
}

func TestIsValid(t *testing.T) {
	t.Run("valid address returns true", func(t *testing.T) {
		addr, _ := Parse("John Smith\n123 Main St\nSYDNEY NSW 2000")
		if !addr.IsValid() {
			t.Error("expected valid address")
		}
	})

	t.Run("address with errors returns false", func(t *testing.T) {
		parser := NewParser(WithStrict(false))
		addr, _ := parser.Parse("John Smith\n123 Main St\nSYDNEY XYZ 2000")
		if addr.IsValid() {
			t.Error("expected invalid address due to invalid state")
		}
	})
}

func TestHasDeliveryPoint(t *testing.T) {
	t.Run("street address has delivery point", func(t *testing.T) {
		addr, _ := Parse("John Smith\n123 Main St\nSYDNEY NSW 2000")
		if !addr.HasDeliveryPoint() {
			t.Error("expected delivery point")
		}
	})

	t.Run("PO Box has delivery point", func(t *testing.T) {
		addr, _ := Parse("Company\nPO Box 1234\nSYDNEY NSW 2000")
		if !addr.HasDeliveryPoint() {
			t.Error("expected delivery point for PO Box")
		}
	})
}

func TestComplexUnitFormats(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		unit   string
		level  string
		number string
	}{
		{
			name:   "apartment prefix",
			input:  "Test\nApartment 5, 100 Main St\nSYDNEY NSW 2000",
			unit:   "APT 5",
			number: "100",
		},
		{
			name:   "flat prefix",
			input:  "Test\nFlat 2, 50 King Rd\nSYDNEY NSW 2000",
			unit:   "FLAT 2",
			number: "50",
		},
		{
			name:   "shop prefix",
			input:  "Test\nShop 1, 200 George St\nSYDNEY NSW 2000",
			unit:   "SHOP 1",
			number: "200",
		},
		{
			name:   "suite prefix",
			input:  "Test\nSuite 10, 300 Pitt St\nSYDNEY NSW 2000",
			unit:   "SUITE 10",
			number: "300",
		},
		{
			name:   "unit with letter",
			input:  "Test\n5A/100 Main St\nSYDNEY NSW 2000",
			unit:   "5A",
			number: "100",
		},
		{
			name:   "level only",
			input:  "Test\nLevel 5, 100 Main St\nSYDNEY NSW 2000",
			level:  "L 5",
			number: "100",
		},
		{
			name:   "floor prefix",
			input:  "Test\nFloor 3, 200 Queen St\nSYDNEY NSW 2000",
			level:  "FL 3",
			number: "200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.unit != "" {
				assertEqual(t, "Unit", tt.unit, addr.Unit)
			}
			if tt.level != "" {
				assertEqual(t, "Level", tt.level, addr.Level)
			}
			if tt.number != "" {
				assertEqual(t, "StreetNumber", tt.number, addr.StreetNumber)
			}
		})
	}
}

func TestStreetTypes(t *testing.T) {
	tests := []struct {
		input      string
		streetType string
	}{
		{"Test\n1 Main Avenue\nSYDNEY NSW 2000", "AV"},
		{"Test\n1 Main Rd\nSYDNEY NSW 2000", "RD"},
		{"Test\n1 Main Boulevard\nSYDNEY NSW 2000", "BVD"},
		{"Test\n1 Main Circuit\nSYDNEY NSW 2000", "CCT"},
		{"Test\n1 Main Highway\nSYDNEY NSW 2000", "HWY"},
		{"Test\n1 Main Esplanade\nSYDNEY NSW 2000", "ESP"},
		{"Test\n1 Main Crescent\nSYDNEY NSW 2000", "CR"},
		{"Test\n1 Main Parade\nSYDNEY NSW 2000", "PDE"},
		{"Test\n1 Main Terrace\nSYDNEY NSW 2000", "TCE"},
		{"Test\n1 Main Way\nSYDNEY NSW 2000", "WAY"},
		{"Test\n1 Main Close\nSYDNEY NSW 2000", "CL"},
		{"Test\n1 Main Court\nSYDNEY NSW 2000", "CT"},
		{"Test\n1 Main Grove\nSYDNEY NSW 2000", "GR"},
	}

	for _, tt := range tests {
		t.Run(tt.streetType, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			assertEqual(t, "StreetType", tt.streetType, addr.StreetType)
		})
	}
}

func TestMultiWordStreetNames(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		streetName string
	}{
		{
			name:       "two word street name",
			input:      "Test\n100 King George Street\nSYDNEY NSW 2000",
			streetName: "KING GEORGE",
		},
		{
			name:       "three word street name",
			input:      "Test\n50 Sir John Young Crescent\nSYDNEY NSW 2000",
			streetName: "SIR JOHN YOUNG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			assertEqual(t, "StreetName", tt.streetName, addr.StreetName)
		})
	}
}

func TestParsePartialAddress(t *testing.T) {
	addr, err := Parse("123 Main Street, Richmond")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "StreetNumber", "123", addr.StreetNumber)
	assertEqual(t, "StreetName", "MAIN", addr.StreetName)
	assertEqual(t, "StreetType", "ST", addr.StreetType)
	assertEqual(t, "Locality", "RICHMOND", addr.Locality)
	assertEqual(t, "State", "", addr.State)
	assertEqual(t, "Postcode", "", addr.Postcode)
	if len(addr.Errors) != 0 {
		t.Fatalf("expected no parsing errors, got %v", addr.Errors)
	}
}

func TestParseSplitAddress(t *testing.T) {
	addr, err := Parse("123 Main\nStreet\nRichmond\nVIC 3121")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "StreetNumber", "123", addr.StreetNumber)
	assertEqual(t, "StreetName", "MAIN", addr.StreetName)
	assertEqual(t, "StreetType", "ST", addr.StreetType)
	assertEqual(t, "Locality", "RICHMOND", addr.Locality)
	assertEqual(t, "State", "VIC", addr.State)
	assertEqual(t, "Postcode", "3121", addr.Postcode)
}

func TestParseMixedDeliveryPoints(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kinds []DeliveryPointKind
	}{
		{
			name:  "street then postal",
			input: "123 Main Street, PO Box 42, Richmond VIC 3121",
			kinds: []DeliveryPointKind{DeliveryPointStreet, DeliveryPointPostal},
		},
		{
			name:  "postal then street",
			input: "Company\nPO Box 42\n123 Main Street\nRichmond VIC 3121",
			kinds: []DeliveryPointKind{DeliveryPointPostal, DeliveryPointStreet},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(addr.DeliveryPoints) != len(tt.kinds) {
				t.Fatalf("expected %d delivery points, got %d: %#v", len(tt.kinds), len(addr.DeliveryPoints), addr.DeliveryPoints)
			}
			for i, kind := range tt.kinds {
				if addr.DeliveryPoints[i].Kind != kind {
					t.Errorf("delivery point %d: expected kind %d, got %d", i, kind, addr.DeliveryPoints[i].Kind)
				}
			}
			assertEqual(t, "StreetNumber", "123", addr.StreetNumber)
			assertEqual(t, "PoBoxType", "PO BOX", addr.PoBoxType)
			assertEqual(t, "PoBoxNumber", "42", addr.PoBoxNumber)
			assertBool(t, "IsPoBox", true, addr.IsPoBox)
			assertEqual(t, "Locality", "RICHMOND", addr.Locality)
			assertEqual(t, "State", "VIC", addr.State)
			assertEqual(t, "Postcode", "3121", addr.Postcode)
			if tt.name == "postal then street" {
				assertStringSliceEqual(t, "NameLines", []string{"Company"}, addr.NameLines)
			}
		})
	}
}

func TestTokenGrammarStrictErrors(t *testing.T) {
	parser := NewParser(WithStrict(true))
	tests := []struct {
		name     string
		input    string
		expected error
	}{
		{"missing locality", "123 Main Street", ErrInvalidAddress},
		{"unknown locality", "123 Main Street, Not A Locality", ErrInvalidAddress},
		{"incompatible state", "123 Main Street, Richmond NT", ErrNoState},
		{"malformed postcode", "123 Main Street, Richmond VIC ABCD", ErrNoPostcode},
		{"trailing input", "123 Main Street, Richmond VIC 3121 EXTRA", ErrInvalidAddress},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.input)
			if err != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, err)
			}
		})
	}
}

func assertEqual(t *testing.T, field, expected, got string) {
	t.Helper()
	if expected != got {
		t.Errorf("%s: expected %q, got %q", field, expected, got)
	}
}

func assertBool(t *testing.T, field string, expected, got bool) {
	t.Helper()
	if expected != got {
		t.Errorf("%s: expected %v, got %v", field, expected, got)
	}
}

func assertStringSliceEqual(t *testing.T, field string, expected, got []string) {
	t.Helper()
	if len(expected) != len(got) {
		t.Errorf("%s: length mismatch, expected %d, got %d", field, len(expected), len(got))
		return
	}
	for i := range expected {
		if expected[i] != got[i] {
			t.Errorf("%s[%d]: expected %q, got %q", field, i, expected[i], got[i])
		}
	}
}
