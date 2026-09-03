package anzaddress

import (
	"regexp"
	"testing"
)

func TestNZLocalityIndexRecordsLINZSnapshot(t *testing.T) {
	if NZLocalitySourceURL == "" || NZLocalityDatasetURL == "" {
		t.Fatal("LINZ source URLs must be recorded")
	}
	if NZLocalityRetrievedAt == "" || NZLocalityReleaseAt == "" {
		t.Fatal("LINZ retrieval and release timestamps must be recorded")
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(NZLocalitySourceSHA256) {
		t.Fatalf("invalid LINZ source checksum: %q", NZLocalitySourceSHA256)
	}
	if NZLocalityLicense != "Creative Commons Attribution 4.0 International" {
		t.Fatalf("unexpected LINZ license: %q", NZLocalityLicense)
	}
	if NZLocalityAttribution == "" {
		t.Fatal("LINZ attribution must be recorded")
	}
	if got, want := NZLocalityFeatureCount, 6_563; got != want {
		t.Fatalf("feature count: got %d, want %d", got, want)
	}
	if got, want := len(nzLocalities), NZLocalityNameCount; got != want {
		t.Fatalf("generated locality count: got %d, want %d", got, want)
	}
	for _, locality := range []string{"AUCKLAND", "ŌTĀHUHU", "KAPITI", "WELLINGTON"} {
		if _, ok := nzLocalities[locality]; !ok {
			t.Errorf("missing LINZ locality %q", locality)
		}
	}
}
