package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParsePeers(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "empty value",
			value: "",
			want:  nil,
		},
		{
			name:  "single peer",
			value: "http://localhost:8081",
			want: []string{
				"http://localhost:8081",
			},
		},
		{
			name:  "multiple peers",
			value: "http://localhost:8081,http://localhost:8082",
			want: []string{
				"http://localhost:8081",
				"http://localhost:8082",
			},
		},
		{
			name:  "spaces and empty entries",
			value: " http://localhost:8081, ,http://localhost:8082,",
			want: []string{
				"http://localhost:8081",
				"http://localhost:8082",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parsePeers(test.value)

			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf(
					"parsePeers(%q) = %#v, want %#v",
					test.value,
					got,
					test.want,
				)
			}
		})
	}
}
func TestParseDuration(t *testing.T) {
	fallback := 5 * time.Second

	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "empty value uses fallback",
			value: "",
			want:  fallback,
		},
		{
			name:  "seconds",
			value: "2s",
			want:  2 * time.Second,
		},
		{
			name:  "milliseconds",
			value: "500ms",
			want:  500 * time.Millisecond,
		},
		{
			name:    "invalid duration",
			value:   "five seconds",
			wantErr: true,
		},
		{
			name:    "zero duration",
			value:   "0s",
			wantErr: true,
		},
		{
			name:    "negative duration",
			value:   "-1s",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDuration(test.value, fallback)

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != test.want {
				t.Fatalf(
					"duration = %v, want %v",
					got,
					test.want,
				)
			}
		})
	}
}
