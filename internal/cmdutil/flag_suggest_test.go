// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
)

// newTriageLikeCmd builds a minimal cobra command whose flag surface mirrors
// `mail +triage` closely enough for suggestion tests: the canonical names
// (--max, --filter, --page-size) plus an inherited persistent flag and a
// hidden flag we want excluded from suggestions.
func newTriageLikeCmd() *cobra.Command {
	root := &cobra.Command{Use: "lark-cli"}
	root.PersistentFlags().String("format", "json", "")

	leaf := &cobra.Command{Use: "+triage", RunE: func(*cobra.Command, []string) error { return nil }}
	leaf.Flags().Int("max", 0, "")
	leaf.Flags().String("filter", "", "")
	leaf.Flags().Int("page-size", 0, "")
	leaf.Flags().String("internal", "", "hidden internal flag")
	_ = leaf.Flags().MarkHidden("internal")
	root.AddCommand(leaf)
	return leaf
}

func TestSuggestFlag_SynonymHit(t *testing.T) {
	cmd := newTriageLikeCmd()
	tests := []struct {
		bad      string
		want     string
		hintHas  string
		wantHint bool
	}{
		{"limit", "max", "did you mean --max?", true},
		{"count", "max", "did you mean --max?", true},
		{"size", "page-size", "did you mean --page-size?", true},
		{"folder", "filter", `--filter '{"folder":"..."}'`, true},
		{"mailbox", "filter", `--filter '{"folder":"..."}'`, true},
		{"subject", "filter", `--filter '{"subject":"..."}'`, true},
	}
	for _, tt := range tests {
		t.Run(tt.bad, func(t *testing.T) {
			got, hint, ok := SuggestFlag(cmd, tt.bad)
			if !ok {
				t.Fatalf("expected suggestion for %q, got none", tt.bad)
			}
			if got != tt.want {
				t.Errorf("suggestion = %q, want %q", got, tt.want)
			}
			if tt.wantHint && !strings.Contains(hint, tt.hintHas) {
				t.Errorf("hint = %q, want substring %q", hint, tt.hintHas)
			}
		})
	}
}

func TestSuggestFlag_SynonymTargetMissingFallsThrough(t *testing.T) {
	// A command without --max: synonym lookup must not blindly return "max".
	root := &cobra.Command{Use: "lark-cli"}
	leaf := &cobra.Command{Use: "+other"}
	leaf.Flags().String("filter", "", "")
	root.AddCommand(leaf)

	if _, _, ok := SuggestFlag(leaf, "limit"); ok {
		t.Error("expected no suggestion when synonym target --max is not registered, got one")
	}
}

func TestSuggestFlag_EditDistanceTypo(t *testing.T) {
	cmd := newTriageLikeCmd()
	cases := map[string]string{
		"filtr":     "filter",
		"fileter":   "filter",
		"page-siez": "page-size", // transposition
	}
	for bad, want := range cases {
		t.Run(bad, func(t *testing.T) {
			got, _, ok := SuggestFlag(cmd, bad)
			if !ok {
				t.Fatalf("expected suggestion for %q, got none", bad)
			}
			if got != want {
				t.Errorf("suggestion for %q = %q, want %q", bad, got, want)
			}
		})
	}
}

func TestSuggestFlag_NoSuggestionWhenFar(t *testing.T) {
	cmd := newTriageLikeCmd()
	if got, _, ok := SuggestFlag(cmd, "xyzabcd"); ok {
		t.Errorf("expected no suggestion for far-distance %q, got %q", "xyzabcd", got)
	}
}

func TestSuggestFlag_HiddenFlagsExcluded(t *testing.T) {
	cmd := newTriageLikeCmd()
	// "internal" is hidden; an obvious typo of it must not be suggested.
	if got, _, ok := SuggestFlag(cmd, "internl"); ok {
		t.Errorf("hidden flag should be excluded from suggestions, got %q", got)
	}
}

func TestSuggestFlag_TiebreakAlphabetical(t *testing.T) {
	// Both "abc" and "abd" are at edit distance 1 from "abe"; tie-breaks
	// alphabetically -> "abc".
	root := &cobra.Command{Use: "lark-cli"}
	leaf := &cobra.Command{Use: "+x"}
	leaf.Flags().String("abc", "", "")
	leaf.Flags().String("abd", "", "")
	root.AddCommand(leaf)
	got, _, ok := SuggestFlag(leaf, "abe")
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got != "abc" {
		t.Errorf("tie-break = %q, want abc", got)
	}
}

func TestSuggestFlag_NilOrEmpty(t *testing.T) {
	if _, _, ok := SuggestFlag(nil, "anything"); ok {
		t.Error("nil cmd should yield no suggestion")
	}
	cmd := newTriageLikeCmd()
	if _, _, ok := SuggestFlag(cmd, ""); ok {
		t.Error("empty badName should yield no suggestion")
	}
}

func TestParseUnknownFlagName(t *testing.T) {
	cases := []struct {
		msg       string
		want      string
		wantShort bool
		wantOK    bool
		comment   string
	}{
		{"unknown flag: --limit", "limit", false, true, "long flag"},
		{"unknown flag: --page-size", "page-size", false, true, "long flag with hyphen"},
		{"unknown shorthand flag: 'x' in -x", "x", true, true, "short flag"},
		{"flag needs an argument: --foo", "", false, false, "different error"},
		{"required flag(s) \"q\" not set", "", false, false, "required flag missing"},
		{"", "", false, false, "empty"},
		{"unknown flag: --", "", false, false, "empty flag name"},
	}
	for _, tc := range cases {
		t.Run(tc.comment, func(t *testing.T) {
			got, isShort, ok := parseUnknownFlagName(tc.msg)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v (msg=%q)", ok, tc.wantOK, tc.msg)
			}
			if got != tc.want {
				t.Errorf("name = %q, want %q (msg=%q)", got, tc.want, tc.msg)
			}
			if isShort != tc.wantShort {
				t.Errorf("isShort = %v, want %v (msg=%q)", isShort, tc.wantShort, tc.msg)
			}
		})
	}
}

func TestUnknownFlagHandler_PassesThroughOtherErrors(t *testing.T) {
	cmd := newTriageLikeCmd()
	original := errors.New("flag needs an argument: --max")
	got := UnknownFlagHandler(cmd, original)
	if got != original {
		t.Errorf("expected pass-through of non-unknown-flag error, got %v", got)
	}
}

func TestUnknownFlagHandler_SynonymProducesEnvelope(t *testing.T) {
	cmd := newTriageLikeCmd()
	got := UnknownFlagHandler(cmd, fmt.Errorf("unknown flag: --limit"))
	var exit *output.ExitError
	if !errors.As(got, &exit) {
		t.Fatalf("expected *output.ExitError, got %T", got)
	}
	if exit.Code != output.ExitValidation {
		t.Errorf("Code = %d, want %d", exit.Code, output.ExitValidation)
	}
	if exit.Detail == nil {
		t.Fatal("Detail is nil")
	}
	if exit.Detail.Type != "validation" {
		t.Errorf("Type = %q, want validation", exit.Detail.Type)
	}
	if exit.Detail.Message != "unknown flag: --limit" {
		t.Errorf("Message = %q, want exact 'unknown flag: --limit'", exit.Detail.Message)
	}
	if exit.Detail.Hint != "did you mean --max?" {
		t.Errorf("Hint = %q, want 'did you mean --max?'", exit.Detail.Hint)
	}
}

func TestUnknownFlagHandler_TypoProducesEnvelope(t *testing.T) {
	cmd := newTriageLikeCmd()
	got := UnknownFlagHandler(cmd, fmt.Errorf("unknown flag: --filtr"))
	var exit *output.ExitError
	if !errors.As(got, &exit) {
		t.Fatalf("expected *output.ExitError, got %T", got)
	}
	if exit.Detail.Hint != "did you mean --filter?" {
		t.Errorf("Hint = %q, want 'did you mean --filter?'", exit.Detail.Hint)
	}
}

func TestUnknownFlagHandler_NoMatchOmitsHint(t *testing.T) {
	cmd := newTriageLikeCmd()
	got := UnknownFlagHandler(cmd, fmt.Errorf("unknown flag: --xyzabcd"))
	var exit *output.ExitError
	if !errors.As(got, &exit) {
		t.Fatalf("expected *output.ExitError, got %T", got)
	}
	if exit.Detail.Hint != "" {
		t.Errorf("Hint = %q, want empty when no suggestion found", exit.Detail.Hint)
	}
	if !strings.Contains(exit.Detail.Message, "--xyzabcd") {
		t.Errorf("Message = %q, want it to include the bad flag name", exit.Detail.Message)
	}
}

func TestUnknownFlagHandler_ShortFlagSkipsSuggestion(t *testing.T) {
	// Short (1-char) flags share no namespace with long flags, so the
	// handler must not Levenshtein-match a single char against long flag
	// names. It should produce a "-X" message and an empty hint.
	cmd := newTriageLikeCmd()
	got := UnknownFlagHandler(cmd, fmt.Errorf("unknown shorthand flag: 'Z' in -Z"))
	var exit *output.ExitError
	if !errors.As(got, &exit) {
		t.Fatalf("expected *output.ExitError, got %T", got)
	}
	if exit.Detail.Message != "unknown flag: -Z" {
		t.Errorf("Message = %q, want 'unknown flag: -Z' (single dash)", exit.Detail.Message)
	}
	if exit.Detail.Hint != "" {
		t.Errorf("Hint = %q, want empty for short flag", exit.Detail.Hint)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"filtr", "filter", 1},
		{"page-siez", "page-size", 2},
		{"limit", "max", 4},
	}
	for _, c := range cases {
		t.Run(c.a+"_"+c.b, func(t *testing.T) {
			if got := levenshtein(c.a, c.b); got != c.want {
				t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}
