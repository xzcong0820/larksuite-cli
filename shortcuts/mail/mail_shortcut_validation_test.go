// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package mail

import (
	"errors"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/output"
)

// assertValidationError fails the test unless err is a *output.ExitError with
// ExitValidation code whose message contains wantSubstr.
func assertValidationError(t *testing.T, err error, wantSubstr string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	var exitErr *output.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *output.ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != output.ExitValidation {
		t.Errorf("expected exit code %d (ExitValidation), got %d", output.ExitValidation, exitErr.Code)
	}
	if exitErr.Detail == nil || exitErr.Detail.Type != "validation" {
		t.Errorf("expected detail type \"validation\", got %+v", exitErr.Detail)
	}
	if wantSubstr != "" && !strings.Contains(exitErr.Error(), wantSubstr) {
		t.Errorf("expected error message to contain %q, got: %v", wantSubstr, exitErr.Error())
	}
}

// assertValidatePasses fails the test if err is a validation error; other
// errors (e.g. API call failures from missing tokens) are acceptable because
// we only care that the Validate callback passed.
func assertValidatePasses(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var exitErr *output.ExitError
	if errors.As(err, &exitErr) && exitErr.Code == output.ExitValidation {
		t.Fatalf("Validate callback should have passed but returned validation error: %v", exitErr)
	}
	// Non-validation errors (auth/API failures) are expected without HTTP mocks.
}

// TC-1: +message --as bot --mailbox me → ErrValidation
func TestMailMessageBotMailboxMeReturnsValidationError(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailMessage, []string{
		"+message", "--as", "bot", "--mailbox", "me", "--message-id", "msg_xxx",
	}, f, stdout)
	assertValidationError(t, err, "does not support --mailbox me")
}

// TC-2: +message --as bot --mailbox explicit → Validate passes
func TestMailMessageBotExplicitMailboxPassesValidation(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailMessage, []string{
		"+message", "--as", "bot", "--mailbox", "alice@example.com", "--message-id", "msg_xxx",
	}, f, stdout)
	assertValidatePasses(t, err)
}

// TC-3: +message --as user --mailbox me → Validate passes
func TestMailMessageUserMailboxMePassesValidation(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailMessage, []string{
		"+message", "--as", "user", "--mailbox", "me", "--message-id", "msg_xxx",
	}, f, stdout)
	assertValidatePasses(t, err)
}

// TC-4: +messages --as bot (default mailbox=me) → ErrValidation
func TestMailMessagesBotDefaultMailboxMeReturnsValidationError(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailMessages, []string{
		"+messages", "--as", "bot", "--message-ids", "msg_xxx",
	}, f, stdout)
	assertValidationError(t, err, "does not support --mailbox me")
}

// TC-5: +messages --as bot --mailbox explicit → Validate passes
func TestMailMessagesBotExplicitMailboxPassesValidation(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailMessages, []string{
		"+messages", "--as", "bot", "--mailbox", "alice@example.com", "--message-ids", "msg_xxx",
	}, f, stdout)
	assertValidatePasses(t, err)
}

// TC-6: +thread --as bot (default mailbox=me) → ErrValidation
func TestMailThreadBotDefaultMailboxMeReturnsValidationError(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailThread, []string{
		"+thread", "--as", "bot", "--thread-id", "thread_xxx",
	}, f, stdout)
	assertValidationError(t, err, "does not support --mailbox me")
}

// TC-7: +thread --as bot --mailbox explicit → Validate passes
func TestMailThreadBotExplicitMailboxPassesValidation(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailThread, []string{
		"+thread", "--as", "bot", "--mailbox", "alice@example.com", "--thread-id", "thread_xxx",
	}, f, stdout)
	assertValidatePasses(t, err)
}

// TC-8: +triage --as bot (default mailbox=me) → ErrValidation
func TestMailTriageBotDefaultMailboxMeReturnsValidationError(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailTriage, []string{
		"+triage", "--as", "bot",
	}, f, stdout)
	assertValidationError(t, err, "does not support --mailbox me")
}

// TC-9: +triage --as bot --mailbox explicit → Validate passes
func TestMailTriageBotExplicitMailboxPassesValidation(t *testing.T) {
	f, stdout, _, _ := mailShortcutTestFactory(t)
	err := runMountedMailShortcut(t, MailTriage, []string{
		"+triage", "--as", "bot", "--mailbox", "alice@example.com",
	}, f, stdout)
	assertValidatePasses(t, err)
}
