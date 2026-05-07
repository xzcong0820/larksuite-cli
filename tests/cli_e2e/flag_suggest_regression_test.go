// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package clie2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFlagSuggest_Regression locks in the structured "did you mean" envelope
// shipped via cmdutil.UnknownFlagHandler. The unit tests in
// internal/cmdutil/flag_suggest_test.go prove the handler logic in isolation;
// this E2E asserts the wiring in cmd/build.go reaches every shortcut and the
// error envelope actually lands on stderr in the contract shape AGENTS.md
// requires (so AI agents can pattern-match `error.hint`).
func TestFlagSuggest_Regression(t *testing.T) {
	setDryRunConfigEnv(t)

	tests := []struct {
		name        string
		args        []string
		wantMessage string
		wantHint    string // empty means: hint must be absent or empty
	}{
		{
			name:        "synonym limit suggests max",
			args:        []string{"mail", "+triage", "--limit", "5", "--dry-run"},
			wantMessage: "unknown flag: --limit",
			wantHint:    "did you mean --max?",
		},
		{
			name:        "synonym folder suggests filter with example",
			args:        []string{"mail", "+triage", "--folder", "Inbox", "--dry-run"},
			wantMessage: "unknown flag: --folder",
			wantHint:    `did you mean --filter? pass folder via --filter '{"folder":"..."}'`,
		},
		{
			name:        "typo filtr suggests filter",
			args:        []string{"mail", "+triage", "--filtr", "{}", "--dry-run"},
			wantMessage: "unknown flag: --filtr",
			wantHint:    "did you mean --filter?",
		},
		{
			name:        "far-distance unknown flag has no hint",
			args:        []string{"mail", "+triage", "--xyzqqq=1", "--dry-run"},
			wantMessage: "unknown flag: --xyzqqq",
			wantHint:    "",
		},
		{
			name:        "synonym not suggested on commands without target flag",
			args:        []string{"auth", "status", "--limit", "1"},
			wantMessage: "unknown flag: --limit",
			wantHint:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			t.Cleanup(cancel)

			result, err := RunCmd(ctx, Request{Args: tc.args})
			require.NoError(t, err)
			result.AssertExitCode(t, 2) // ExitValidation

			var env map[string]any
			require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(result.Stderr)), &env),
				"stderr must be valid JSON; got:\n%s", result.Stderr)
			assert.Equal(t, false, env["ok"], "ok should be false")

			errObj, ok := env["error"].(map[string]any)
			require.True(t, ok, "error must be object: %#v", env)
			assert.Equal(t, "validation", errObj["type"])
			assert.Equal(t, tc.wantMessage, errObj["message"])

			hint, _ := errObj["hint"].(string)
			assert.Equal(t, tc.wantHint, hint)
		})
	}
}

// TestFlagSuggest_RegressionPassThrough proves the handler does NOT swallow
// other flag-error types (required flag missing, value type mismatch). They
// must reach cobra's default plain-text "Error: ..." path so we don't change
// existing semantics for non-unknown-flag errors.
func TestFlagSuggest_RegressionPassThrough(t *testing.T) {
	setDryRunConfigEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	result, err := RunCmd(ctx, Request{
		Args: []string{"mail", "+triage", "--max", "not-a-number", "--dry-run"},
	})
	require.NoError(t, err)
	result.AssertExitCode(t, 1)
	combined := result.Stdout + result.Stderr
	assert.Contains(t, combined, "invalid argument", "type-mismatch error must reach the user")
	// And it must NOT be wrapped in our did-you-mean envelope.
	assert.NotContains(t, combined, `"hint":"did you mean`)
}
