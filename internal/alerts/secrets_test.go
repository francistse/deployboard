package alerts

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests use a throwaway Keychain FILE and clean up after themselves. They
// are skipped when /usr/bin/security is unavailable.
//
// Why a separate keychain: writing to the login keychain from a test binary
// makes macOS ask the user for a password (each `go test` run is a new process
// to the item's ACL). A temporary keychain with an empty password takes the user
// out of the loop entirely, and nothing of theirs is touched.
const (
	testSvc = "deployboard-selftest"
	testAcc = "token"
)

func keychainAvailable() bool {
	return exec.Command(securityBin, "-h").Run() == nil
}

// useThrowawayKeychain creates an empty-password keychain in a temp dir and
// points the package's keychain calls at it for the duration of the test.
func useThrowawayKeychain(t *testing.T) {
	t.Helper()
	if !keychainAvailable() {
		t.Skip("security(1) unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "selftest.keychain-db")
	if out, err := exec.Command(securityBin, "create-keychain", "-p", "", path).CombinedOutput(); err != nil {
		t.Skipf("cannot create a throwaway keychain (%v: %s)", err, strings.TrimSpace(string(out)))
	}
	exec.Command(securityBin, "unlock-keychain", "-p", "", path).Run()
	// No auto-lock: a keychain that locks mid-test would make security(1) block on
	// a password prompt, which is exactly what this helper exists to avoid.
	if out, err := exec.Command(securityBin, "set-keychain-settings", path).CombinedOutput(); err != nil {
		t.Skipf("cannot disable auto-lock on the throwaway keychain (%v: %s)", err, strings.TrimSpace(string(out)))
	}
	t.Setenv(KeychainEnvVar, path)
	t.Cleanup(func() {
		exec.Command(securityBin, "delete-keychain", path).Run()
	})
}

func TestKeychainStoreReadDeleteRoundTrip(t *testing.T) {
	useThrowawayKeychain(t)
	t.Cleanup(func() { _ = DeleteKeychainToken(testSvc, testAcc, 5*time.Second) })

	const secret = "123456789:TEST-TOKEN-NOT-REAL"

	if err := StoreKeychainToken(testSvc, testAcc, secret, 6*time.Second); err != nil {
		// A locked login keychain cannot be written headlessly, and only the
		// user can unlock it — skip rather than fail the suite.
		if strings.Contains(err.Error(), "locked") {
			t.Skip("login keychain is locked; unlock it to exercise the write path")
		}
		t.Fatalf("StoreKeychainToken: %v", err)
	}
	got, err := ReadKeychainToken(testSvc, testAcc, 5*time.Second)
	if err != nil {
		t.Fatalf("ReadKeychainToken: %v", err)
	}
	if got != secret {
		t.Errorf("round trip mismatch: got %q", got)
	}
	if !KeychainTokenReadable(testSvc, testAcc, 5*time.Second) {
		t.Error("KeychainTokenReadable should be true after storing")
	}

	// Idempotent overwrite (security add-generic-password -U).
	if err := StoreKeychainToken(testSvc, testAcc, secret+"-rotated", 10*time.Second); err != nil {
		t.Fatalf("StoreKeychainToken (rotate): %v", err)
	}
	got2, _ := ReadKeychainToken(testSvc, testAcc, 5*time.Second)
	if got2 != secret+"-rotated" {
		t.Errorf("rotation failed, got %q", got2)
	}

	if err := DeleteKeychainToken(testSvc, testAcc, 5*time.Second); err != nil {
		t.Fatalf("DeleteKeychainToken: %v", err)
	}
	if KeychainTokenReadable(testSvc, testAcc, 5*time.Second) {
		t.Error("item should be gone after delete")
	}
	// Deleting again is a no-op, not an error.
	if err := DeleteKeychainToken(testSvc, testAcc, 5*time.Second); err != nil {
		t.Errorf("second delete should be a no-op, got %v", err)
	}
}

func TestStoreKeychainToken_RejectsEmpty(t *testing.T) {
	if err := StoreKeychainToken(testSvc, testAcc, "   ", time.Second); err == nil {
		t.Error("empty token should be rejected")
	}
	if err := StoreKeychainToken("", "", "x", time.Second); err == nil {
		t.Error("empty service/account should be rejected")
	}
}

func TestReadKeychainToken_MissingIsError(t *testing.T) {
	if !keychainAvailable() {
		t.Skip("security(1) unavailable")
	}
	_, err := ReadKeychainToken("deployboard-definitely-missing", "nope", 5*time.Second)
	if err == nil {
		t.Fatal("expected an error for a missing item")
	}
	if !strings.Contains(err.Error(), "keychain") {
		t.Errorf("error should mention the keychain: %v", err)
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("secret-token", "failed with secret-token in the message"); strings.Contains(got, "secret-token") {
		t.Errorf("redaction failed: %q", got)
	}
	if got := Redact("", "unchanged"); got != "unchanged" {
		t.Errorf("empty secret should not change the string: %q", got)
	}
}

func TestEngine_ConfigureWithoutTokenDisables(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Telegram.KeychainService = "deployboard-definitely-missing"
	cfg.Telegram.KeychainAccount = "nope"
	e := NewEngine(cfg, NewState(""))

	if e.Enabled() {
		t.Error("engine must be disabled when the keychain item is missing")
	}
	if e.WarnOnce() == "" {
		t.Error("WarnOnce should explain why alerts are off")
	}
	if e.WarnOnce() != "" {
		t.Error("WarnOnce should only speak once")
	}
}

func TestEngine_SetChatID(t *testing.T) {
	e := NewEngine(DefaultConfig(), NewState(""))
	e.SetChatID("12345")
	if got := e.TelegramConfig().ChatID; got != "12345" {
		t.Errorf("chat id = %q, want 12345", got)
	}
}
