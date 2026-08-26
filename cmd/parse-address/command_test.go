package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunParse(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "formatted address",
			args:       []string{"parse-address", "Level 4, 54 Wellington Street, Collingwood"},
			wantStdout: "L 4 54 WELLINGTON ST\nCOLLINGWOOD\n",
		},
		{
			name:       "missing address",
			args:       []string{"parse-address"},
			wantCode:   1,
			wantStderr: "expected one address input",
		},
		{
			name:       "too many addresses",
			args:       []string{"parse-address", "54 Wellington Street, Collingwood", "1 George Street, Sydney"},
			wantCode:   1,
			wantStderr: "expected one address input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Fatalf("exit code: want %d, got %d", tt.wantCode, code)
			}
			if stdout.String() != tt.wantStdout {
				t.Errorf("stdout:\nwant: %q\ngot:  %q", tt.wantStdout, stdout.String())
			}
			if tt.wantStderr == "" {
				if stderr.Len() != 0 {
					t.Errorf("stderr: want empty, got %q", stderr.String())
				}
			} else if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr: want substring %q, got %q", tt.wantStderr, stderr.String())
			}
		})
	}
}

func TestRunParseMultiple(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "actual newline",
			input: "Level 4, 54 Wellington St, Collingwood\nPO Box 234, Melbourne",
		},
		{
			name:  "literal newline",
			input: "Level 4, 54 Wellington St, Collingwood\\nPO Box 234, Melbourne",
		},
	}
	want := "L 4 54 WELLINGTON ST\nCOLLINGWOOD\n\nPO BOX 234\nMELBOURNE\n"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(
				context.Background(),
				[]string{"parse-address", tt.input},
				&stdout,
				&stderr,
			)

			if code != 0 {
				t.Fatalf("exit code: want 0, got %d; stderr: %s", code, stderr.String())
			}
			if stdout.String() != want {
				t.Errorf("stdout:\nwant: %q\ngot:  %q", want, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr: want empty, got %q", stderr.String())
			}
		})
	}
}

func TestRunParseMultipleFailureIsAtomic(t *testing.T) {
	const input = "54 Wellington Street, Collingwood\n54 Imaginary Street"

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "plain",
			args:       []string{"parse-address", input},
			wantStderr: "invalid address format\n",
		},
		{
			name:       "JSON",
			args:       []string{"parse-address", input, "--json"},
			wantStderr: `{"error":"invalid address format"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code: want 1, got %d", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout: want empty, got %q", stdout.String())
			}
			if stderr.String() != tt.wantStderr {
				t.Errorf("stderr:\nwant: %q\ngot:  %q", tt.wantStderr, stderr.String())
			}
		})
	}
}

func TestRunParseJSON(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "flag after address",
			args: []string{"parse-address", "Level 4, 54 Wellington Street, Collingwood", "--json"},
			want: `[{"deliveryPoints":[{"kind":"street","level":"L 4","streetNumber":"54","streetName":"WELLINGTON","streetType":"ST"}],"locality":"COLLINGWOOD"}]` + "\n",
		},
		{
			name: "flag before address",
			args: []string{"parse-address", "--json", "Level 4, 54 Wellington Street, Collingwood"},
			want: `[{"deliveryPoints":[{"kind":"street","level":"L 4","streetNumber":"54","streetName":"WELLINGTON","streetType":"ST"}],"locality":"COLLINGWOOD"}]` + "\n",
		},
		{
			name: "postal delivery",
			args: []string{"parse-address", "PO Box 42, Richmond", "--json"},
			want: `[{"deliveryPoints":[{"kind":"postal","postalType":"PO BOX","postalNumber":"42"}],"locality":"RICHMOND"}]` + "\n",
		},
		{
			name: "multiple addresses",
			args: []string{
				"parse-address",
				"Level 4, 54 Wellington St, Collingwood\\nPO Box 234, Melbourne",
				"--json",
			},
			want: `[{"deliveryPoints":[{"kind":"street","level":"L 4","streetNumber":"54","streetName":"WELLINGTON","streetType":"ST"}],"locality":"COLLINGWOOD"},{"deliveryPoints":[{"kind":"postal","postalType":"PO BOX","postalNumber":"234"}],"locality":"MELBOURNE"}]` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code: want 0, got %d; stderr: %s", code, stderr.String())
			}
			if stdout.String() != tt.want {
				t.Errorf("stdout:\nwant: %q\ngot:  %q", tt.want, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr: want empty, got %q", stderr.String())
			}
		})
	}
}

func TestRunCompare(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "exact",
			args: []string{
				"parse-address", "compare",
				"54 Wellington Street, Collingwood VIC 3066",
				"54 Wellington St, Collingwood VIC 3066",
			},
			want: "exact\n",
		},
		{
			name: "partial",
			args: []string{
				"parse-address", "compare",
				"54 Wellington Street, Collingwood",
				"54 Wellington St, Collingwood VIC 3066",
			},
			want: "partial\nmatched through: locality\nmissing from left: state, postcode\n",
		},
		{
			name: "no match",
			args: []string{
				"parse-address", "compare",
				"54 Wellington Street, Collingwood",
				"55 Wellington Street, Collingwood",
			},
			want: "no match\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code: want 0, got %d; stderr: %s", code, stderr.String())
			}
			if stdout.String() != tt.want {
				t.Errorf("stdout:\nwant: %q\ngot:  %q", tt.want, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr: want empty, got %q", stderr.String())
			}
		})
	}
}

func TestRunCompareJSON(t *testing.T) {
	args := []string{
		"parse-address", "compare",
		"54 Wellington Street, Collingwood",
		"54 Wellington St, Collingwood VIC 3066",
		"--json",
	}
	want := `{"kind":"partial","matchedThrough":"locality","missingFromLeft":["state","postcode"],"missingFromRight":[],"leftKey":"STREET{UNIT=;LEVEL=;NUMBER=54;NAME=WELLINGTON;TYPE=ST;SUFFIX=}|LOCALITY=COLLINGWOOD|STATE=|POSTCODE=","rightKey":"STREET{UNIT=;LEVEL=;NUMBER=54;NAME=WELLINGTON;TYPE=ST;SUFFIX=}|LOCALITY=COLLINGWOOD|STATE=VIC|POSTCODE=3066"}` + "\n"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), args, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code: want 0, got %d; stderr: %s", code, stderr.String())
	}
	if stdout.String() != want {
		t.Errorf("stdout:\nwant: %q\ngot:  %q", want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr: want empty, got %q", stderr.String())
	}
}

func TestRunErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"parse-address", "54 Imaginary Street"},
		&stdout,
		&stderr,
	)

	if code != 1 {
		t.Fatalf("exit code: want 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout: want empty, got %q", stdout.String())
	}
	if stderr.String() != "invalid address format\n" {
		t.Errorf("stderr: want %q, got %q", "invalid address format\n", stderr.String())
	}
}

func TestRunJSONErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "invalid address",
			args: []string{"parse-address", "54 Imaginary Street", "--json"},
			want: `{"error":"invalid address format"}` + "\n",
		},
		{
			name: "invalid left address",
			args: []string{
				"parse-address", "compare", "54 Imaginary Street",
				"54 Wellington Street, Collingwood", "--json",
			},
			want: `{"error":"left address: invalid address format"}` + "\n",
		},
		{
			name: "invalid right address",
			args: []string{
				"parse-address", "compare", "54 Wellington Street, Collingwood",
				"54 Imaginary Street", "--json",
			},
			want: `{"error":"right address: invalid address format"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code: want 1, got %d", code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout: want empty, got %q", stdout.String())
			}
			if stderr.String() != tt.want {
				t.Errorf("stderr:\nwant: %q\ngot:  %q", tt.want, stderr.String())
			}
		})
	}
}
