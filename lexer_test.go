package auaddress

import "testing"

func TestLexAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []tokenSummary
	}{
		{
			name:  "slash range and soft boundaries",
			input: "3A/10-12 Main\nStreet, Richmond",
			expected: []tokenSummary{
				{tokenNumberish, "3A"},
				{tokenSlash, "/"},
				{tokenNumberish, "10-12"},
				{tokenWord, "MAIN"},
				{tokenNewline, "\n"},
				{tokenWord, "STREET"},
				{tokenComma, ","},
				{tokenWord, "RICHMOND"},
				{tokenEOF, ""},
			},
		},
		{
			name:  "Windows lines and punctuated postal type",
			input: "P.O. Box 42\r\nNorth Sydney",
			expected: []tokenSummary{
				{tokenWord, "PO"},
				{tokenWord, "BOX"},
				{tokenNumberish, "42"},
				{tokenNewline, "\n"},
				{tokenWord, "NORTH"},
				{tokenWord, "SYDNEY"},
				{tokenEOF, ""},
			},
		},
		{
			name:  "Unicode and apostrophe",
			input: "1 O'Connell Street",
			expected: []tokenSummary{
				{tokenNumberish, "1"},
				{tokenWord, "O'CONNELL"},
				{tokenWord, "STREET"},
				{tokenEOF, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexAddress(tt.input)
			if err != nil {
				t.Fatalf("lexAddress returned an error: %v", err)
			}

			actual := summarizeTokens(tokens)
			if len(actual) != len(tt.expected) {
				t.Fatalf("token count: expected %d, got %d: %#v", len(tt.expected), len(actual), actual)
			}
			for i := range tt.expected {
				if actual[i] != tt.expected[i] {
					t.Errorf("token %d: expected %#v, got %#v", i, tt.expected[i], actual[i])
				}
			}
		})
	}
}

func TestLexAddressTracksSourcePosition(t *testing.T) {
	tokens, err := lexAddress("Main\nStreet")
	if err != nil {
		t.Fatalf("lexAddress returned an error: %v", err)
	}

	if tokens[0].line != 1 || tokens[0].column != 1 || tokens[0].offset != 0 {
		t.Errorf("first token position: got line=%d column=%d offset=%d", tokens[0].line, tokens[0].column, tokens[0].offset)
	}
	if tokens[2].line != 2 || tokens[2].column != 1 {
		t.Errorf("third token position: got line=%d column=%d", tokens[2].line, tokens[2].column)
	}
}

type tokenSummary struct {
	kind  tokenKind
	value string
}

func summarizeTokens(tokens []token) []tokenSummary {
	result := make([]tokenSummary, len(tokens))
	for i, token := range tokens {
		result[i] = tokenSummary{kind: token.kind, value: token.value}
	}
	return result
}
