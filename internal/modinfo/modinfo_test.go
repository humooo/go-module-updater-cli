package modinfo

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantModule string
		wantGo     string
		wantErr    string
	}{
		{
			name:       "valid",
			input:      "module example.com/foo\n\ngo 1.21\n\nrequire golang.org/x/mod v0.17.0\n",
			wantModule: "example.com/foo",
			wantGo:     "1.21",
		},
		{
			name:    "invalid_syntax",
			input:   "not a go.mod file at all!!!",
			wantErr: "parse go.mod",
		},
		{
			name:    "missing_module",
			input:   "go 1.21\n",
			wantErr: "go.mod has no module or go directive",
		},
		{
			name:    "missing_go",
			input:   "module example.com/foo\n",
			wantErr: "go.mod has no module or go directive",
		},
		{
			name:    "empty_input",
			input:   "",
			wantErr: "go.mod has no module or go directive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := Parse([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Module != tt.wantModule {
				t.Errorf("Module = %q, want %q", info.Module, tt.wantModule)
			}
			if info.GoVersion != tt.wantGo {
				t.Errorf("GoVersion = %q, want %q", info.GoVersion, tt.wantGo)
			}
		})
	}
}
