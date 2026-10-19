package main

import (
	"io"
	"math"
	"strings"
	"testing"
)

func TestContrast(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		want    float64
		wantErr bool
	}{
		{name: "black on white", a: "#000000", b: "#FFFFFF", want: 21},
		{name: "order does not matter", a: "#FFFFFF", b: "#000000", want: 21},
		{name: "same colour", a: "#777777", b: "#777777", want: 1},
		{name: "grey on white", a: "#767676", b: "#FFFFFF", want: 4.54},
		{name: "not hex", a: "red", b: "#FFFFFF", wantErr: true},
		{name: "short hex", a: "#FFF", b: "#FFFFFF", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := contrast(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("contrast(%q, %q) error = %v, wantErr %v", tt.a, tt.b, err, tt.wantErr)
			}
			if !tt.wantErr && math.Abs(got-tt.want) > 0.01 {
				t.Errorf("contrast(%q, %q) = %.3f, want %.2f", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name: "passing pairing",
			json: `{"colors":{"fg":{"light":"#000000","dark":"#FFFFFF"},"bg":{"light":"#FFFFFF","dark":"#000000"}},
				"pairings":[{"foreground":"fg","background":"bg"}]}`,
		},
		{
			name: "dark fails",
			json: `{"colors":{"fg":{"light":"#000000","dark":"#333333"},"bg":{"light":"#FFFFFF","dark":"#000000"}},
				"pairings":[{"foreground":"fg","background":"bg"}]}`,
			wantErr: "fg on bg (dark)",
		},
		{
			name: "unknown role",
			json: `{"colors":{"fg":{"light":"#000000","dark":"#FFFFFF"}},
				"pairings":[{"foreground":"fg","background":"missing"}]}`,
			wantErr: "unknown colour role",
		},
		{
			name:    "no pairings",
			json:    `{"colors":{}}`,
			wantErr: "no pairings",
		},
		{
			name:    "not json",
			json:    `{`,
			wantErr: "tokens:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := check([]byte(tt.json), io.Discard)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("check: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("check error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
