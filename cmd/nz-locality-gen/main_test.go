package main

import (
	"strings"
	"testing"
)

func TestRenderIncludesCanonicalAndAlternateLINZNamesDeterministically(t *testing.T) {
	source := []byte(`{
		"features": [
			{"attributes": {"name": "Ōtāhuhu", "additional_name": "Otahuhu", "name_ascii": "Otahuhu", "additional_name_ascii": ""}},
			{"attributes": {"name": "Auckland", "major_name": "Auckland", "major_name_ascii": "Auckland"}}
		]
	}`)

	first, err := render(source)
	if err != nil {
		t.Fatalf("render returned an error: %v", err)
	}
	second, err := render(source)
	if err != nil {
		t.Fatalf("second render returned an error: %v", err)
	}
	if got, want := string(second), string(first); got != want {
		t.Fatalf("render is not deterministic:\nfirst: %s\nsecond: %s", first, second)
	}

	generated := string(first)
	for _, locality := range []string{"\"AUCKLAND\"", "\"OTAHUHU\"", "\"ŌTĀHUHU\""} {
		if !strings.Contains(generated, locality) {
			t.Errorf("generated index does not include %s:\n%s", locality, generated)
		}
	}
}
