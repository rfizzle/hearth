package main

import (
	"strings"
	"testing"
)

const sample = `{"colors":{
"a.b":{"light":"#112233","dark":"#445566"},
"chart.1":{"light":"#000001","dark":"#000002"},"chart.2":{"light":"#000003","dark":"#000004"},
"chart.3":{"light":"#000005","dark":"#000006"},"chart.4":{"light":"#000007","dark":"#000008"},
"chart.5":{"light":"#000009","dark":"#00000A"},"chart.6":{"light":"#00000B","dark":"#00000C"}}}`

func TestGenerate(t *testing.T) {
	css, palette, err := generate([]byte(sample))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{"--color-a-b: #112233;", "--color-a-b: #445566;", "--spacing-xs: 4px;", "--radius-lg: 16px;", "prefers-color-scheme: dark"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("theme.css lacks %q", want)
		}
	}
	for _, want := range []string{"DO NOT EDIT", `Chart1Light = "#000001"`, `Chart6Dark  = "#00000C"`} {
		if !strings.Contains(string(palette), want) {
			t.Errorf("palette.go lacks %q", want)
		}
	}
	css2, palette2, _ := generate([]byte(sample))
	if string(css) != string(css2) || string(palette) != string(palette2) {
		t.Error("generate is not deterministic")
	}
}

func TestGenerateRefuses(t *testing.T) {
	for name, in := range map[string]string{
		"not json":     `{`,
		"bad hex":      `{"colors":{"a":{"light":"red","dark":"#000000"}}}`,
		"missing hues": `{"colors":{"a":{"light":"#000000","dark":"#000000"}}}`,
	} {
		if _, _, err := generate([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
