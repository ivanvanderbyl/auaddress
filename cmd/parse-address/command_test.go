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
			wantStderr: "expected one address",
		},
		{
			name:       "too many addresses",
			args:       []string{"parse-address", "54 Wellington Street, Collingwood", "1 George Street, Sydney"},
			wantCode:   1,
			wantStderr: "expected one address",
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
