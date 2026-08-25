package main

import (
	"encoding/json"
	"testing"
)

func TestQueryResponseAcceptsStringStateCode(t *testing.T) {
	payload := []byte(`{
        "features": [{
            "attributes": {
                "sal_name_2021": "Aarons Pass",
                "state_code_2021": "1"
            }
        }]
    }`)

	var response queryResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode ABS response: %v", err)
	}
	if got := response.Features[0].Attributes.StateCode; got != "1" {
		t.Errorf("state code: expected %q, got %q", "1", got)
	}
}
