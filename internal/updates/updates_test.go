package updates

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []DepUpdate
		wantErr bool
	}{
		{
			name:  "empty_input",
			input: "",
			want:  nil,
		},
		{
			name:  "main_skipped",
			input: `{"Path":"example.com/foo","Version":"v0.0.0","Main":true}`,
			want:  nil,
		},
		{
			name:  "no_update",
			input: `{"Path":"dep.example/a","Version":"v1.0.0"}`,
			want:  nil,
		},
		{
			name:  "empty_update_version",
			input: `{"Path":"dep.example/a","Version":"v1.0.0","Update":{"Path":"dep.example/a","Version":""}}`,
			want:  nil,
		},
		{
			name:  "single_update",
			input: `{"Path":"dep.example/a","Version":"v1.0.0","Update":{"Path":"dep.example/a","Version":"v1.1.0"}}`,
			want: []DepUpdate{
				{Path: "dep.example/a", Current: "v1.0.0", Latest: "v1.1.0", Indirect: false},
			},
		},
		{
			name: "mixed",
			input: `{"Path":"example.com/foo","Version":"v0.0.0","Main":true}
{"Path":"dep.example/a","Version":"v1.0.0"}
{"Path":"dep.example/b","Version":"v2.0.0","Update":{"Path":"dep.example/b","Version":"v2.3.0"}}`,
			want: []DepUpdate{
				{Path: "dep.example/b", Current: "v2.0.0", Latest: "v2.3.0", Indirect: false},
			},
		},
		{
			name:  "indirect_flag",
			input: `{"Path":"dep.example/c","Version":"v0.5.0","Indirect":true,"Update":{"Path":"dep.example/c","Version":"v0.6.0"}}`,
			want: []DepUpdate{
				{Path: "dep.example/c", Current: "v0.5.0", Latest: "v0.6.0", Indirect: true},
			},
		},
		{
			name: "sorted_output",
			input: `{"Path":"dep.example/z","Version":"v1.0.0","Update":{"Path":"dep.example/z","Version":"v1.2.0"}}
{"Path":"dep.example/a","Version":"v0.1.0","Update":{"Path":"dep.example/a","Version":"v0.2.0"}}`,
			want: []DepUpdate{
				{Path: "dep.example/a", Current: "v0.1.0", Latest: "v0.2.0", Indirect: false},
				{Path: "dep.example/z", Current: "v1.0.0", Latest: "v1.2.0", Indirect: false},
			},
		},
		{
			name:    "invalid_json",
			input:   `{`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
