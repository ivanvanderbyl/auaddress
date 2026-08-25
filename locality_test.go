package auaddress

import "testing"

func TestMatchLocalityLongest(t *testing.T) {
	tokens, err := lexAddress("ALICE\nSPRINGS NT")
	if err != nil {
		t.Fatalf("lexAddress returned an error: %v", err)
	}

	match, ok := matchLocality(tokens, 0)
	if !ok {
		t.Fatal("expected a locality match")
	}
	assertEqual(t, "name", "ALICE SPRINGS", match.name)
	if !match.states.contains("NT") {
		t.Errorf("expected ALICE SPRINGS state mask to contain NT: %08b", match.states)
	}
	if match.next != 3 {
		t.Errorf("expected match to consume through token 3, got %d", match.next)
	}
}

func TestLocalityStateMaskMergesDuplicateNames(t *testing.T) {
	states, ok := localityStates["RICHMOND"]
	if !ok {
		t.Fatal("expected RICHMOND in locality index")
	}

	for _, state := range []string{"NSW", "VIC", "QLD", "SA", "TAS"} {
		if !states.contains(state) {
			t.Errorf("expected RICHMOND state mask to contain %s: %08b", state, states)
		}
	}
	if states.contains("NT") {
		t.Errorf("did not expect RICHMOND state mask to contain NT: %08b", states)
	}
}

func TestLocalityIndexHasOfficialData(t *testing.T) {
	if len(localityStates) < 13_000 || len(localityStates) > 15_000 {
		t.Errorf("unexpected locality index size: %d", len(localityStates))
	}
	if maxLocalityTokens < 3 {
		t.Errorf("maximum locality token count is unexpectedly small: %d", maxLocalityTokens)
	}
}
