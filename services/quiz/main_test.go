package main

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	csv := "id,domain,prompt,option_a,option_b,option_c,option_d,answer,explanation,source\nq1,Twig,Safe output?,Escape,Eval,Delete,Cache,0,Twig escapes,https://example.com\n"
	questions, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 1 || questions[0].Answer != 0 || questions[0].Options[0] != "Escape" {
		t.Fatalf("unexpected questions: %#v", questions)
	}
}

func TestParseCSVRejectsBadAnswer(t *testing.T) {
	csv := "id,domain,prompt,option_a,option_b,option_c,option_d,answer,explanation,source\nq1,Twig,Safe output?,A,B,C,D,8,Why,https://example.com\n"
	if _, err := parseCSV(strings.NewReader(csv)); err == nil {
		t.Fatal("expected invalid answer error")
	}
}
