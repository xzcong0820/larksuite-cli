// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"fmt"
	"sort"
	"strings"

	"github.com/larksuite/cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// commonSynonyms maps wrong flag names that come from other CLIs' conventions
// to the canonical lark-cli name. It is consulted before edit-distance
// matching because the names are far apart in Levenshtein distance (e.g.
// "limit" vs "max" is distance 4) but semantically interchangeable in agent
// prompts. A synonym is only suggested when the canonical target flag is
// actually registered on the invoked command.
var commonSynonyms = map[string]synonymHint{
	"limit":   {target: "max"},
	"count":   {target: "max"},
	"size":    {target: "page-size"},
	"folder":  {target: "filter", extra: `pass folder via --filter '{"folder":"..."}'`},
	"mailbox": {target: "filter", extra: `pass mailbox via --filter '{"folder":"..."}'`},
	"subject": {target: "filter", extra: `pass subject via --filter '{"subject":"..."}'`},
}

type synonymHint struct {
	target string
	extra  string
}

// UnknownFlagHandler is wired via rootCmd.SetFlagErrorFunc. cobra walks up to
// the root when a child command has not set its own FlagErrorFunc, so a
// single install on the root command applies CLI-wide.
//
// It only acts on "unknown flag" parse errors. Every other flag-error type
// (required flag missing, value type mismatch, ambiguous flag, etc.) is
// returned unchanged so cobra's default semantics apply.
//
// On a hit, it returns an *output.ExitError with type=validation. The root
// error handler in cmd/root.go routes ExitError through
// output.WriteErrorEnvelope, producing the structured stderr JSON that
// AGENTS.md mandates for AI-agent consumption.
func UnknownFlagHandler(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	badName, isShort, ok := parseUnknownFlagName(err.Error())
	if !ok {
		return err
	}
	dashes := "--"
	if isShort {
		dashes = "-"
	}
	exit := output.ErrValidation("unknown flag: %s%s", dashes, badName)
	// Skip suggestions for short (single-char) flags: pflag's short-flag
	// namespace is completely separate from long flags, and 1-char inputs
	// would Levenshtein-match almost anything within the default threshold.
	if isShort {
		return exit
	}
	if suggestion, hint, found := SuggestFlag(cmd, badName); found {
		if hint == "" {
			hint = fmt.Sprintf("did you mean --%s?", suggestion)
		}
		exit.Detail.Hint = hint
	}
	return exit
}

// parseUnknownFlagName extracts the bad flag name from a pflag error message.
// pflag emits "unknown flag: --foo" for long flags and
// "unknown shorthand flag: 'x' in -x" for short flags. The second return
// value reports whether it was a shorthand. ok=false signals the caller to
// pass the error through untouched.
func parseUnknownFlagName(msg string) (name string, isShort bool, ok bool) {
	const longPrefix = "unknown flag: --"
	const shortPrefix = "unknown shorthand flag: '"
	switch {
	case strings.HasPrefix(msg, longPrefix):
		n := strings.TrimPrefix(msg, longPrefix)
		if i := strings.IndexAny(n, " \t\n"); i >= 0 {
			n = n[:i]
		}
		return n, false, n != ""
	case strings.HasPrefix(msg, shortPrefix):
		rest := strings.TrimPrefix(msg, shortPrefix)
		if end := strings.Index(rest, "'"); end >= 0 {
			n := rest[:end]
			return n, true, n != ""
		}
	}
	return "", false, false
}

// SuggestFlag returns the closest flag name on cmd to badName.
//
// Layer A (synonyms): consults commonSynonyms first. The synonym is only
// returned when the target flag is registered (and not hidden) on cmd, so we
// don't mislead users on commands that don't define it.
//
// Layer B (edit distance): Levenshtein with threshold max(2, len(badName)/3).
// Hidden flags are excluded.
func SuggestFlag(cmd *cobra.Command, badName string) (suggestion string, hint string, ok bool) {
	if cmd == nil || badName == "" {
		return "", "", false
	}
	flags := cmd.Flags()

	if syn, found := commonSynonyms[badName]; found {
		if f := flags.Lookup(syn.target); f != nil && !f.Hidden {
			hintMsg := fmt.Sprintf("did you mean --%s?", syn.target)
			if syn.extra != "" {
				hintMsg = hintMsg + " " + syn.extra
			}
			return syn.target, hintMsg, true
		}
	}

	threshold := len(badName) / 3
	if threshold < 2 {
		threshold = 2
	}
	type cand struct {
		name string
		d    int
	}
	var cands []cand
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		d := levenshtein(badName, f.Name)
		if d <= threshold {
			cands = append(cands, cand{f.Name, d})
		}
	})
	if len(cands) == 0 {
		return "", "", false
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].d != cands[j].d {
			return cands[i].d < cands[j].d
		}
		return cands[i].name < cands[j].name
	})
	return cands[0].name, "", true
}

// levenshtein returns the edit distance between a and b using a single-row
// DP. Inlined to avoid pulling a third-party dependency for ~25 lines.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	ar := []rune(a)
	br := []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// Compile-time guard: signature must match cobra's FlagErrorFunc.
var _ func(*cobra.Command, error) error = UnknownFlagHandler
