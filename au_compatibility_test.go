package anzaddress

import "testing"

func TestAustralianNormalFormAndComparisonKeyRemainStable(t *testing.T) {
	address, err := Parse("Level 4, 54 Wellington Street, Collingwood VIC 3066")
	if err != nil {
		t.Fatalf("parse Australian address: %v", err)
	}

	if got, want := address.Format(), "L 4 54 WELLINGTON ST\nCOLLINGWOOD VIC 3066"; got != want {
		t.Fatalf("normal form: want %q, got %q", want, got)
	}
	if got, want := address.ComparisonKey(), "STREET{UNIT=;LEVEL=L 4;NUMBER=54;NAME=WELLINGTON;TYPE=ST;SUFFIX=}|LOCALITY=COLLINGWOOD|STATE=VIC|POSTCODE=3066"; got != want {
		t.Fatalf("comparison key: want %q, got %q", want, got)
	}
}
