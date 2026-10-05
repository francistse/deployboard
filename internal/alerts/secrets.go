package alerts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StoreKeychainToken writes (or replaces) the Telegram bot token in the macOS
// Keychain, whitelisting /usr/bin/security as the reader so later headless reads
// never prompt for authorisation.
//
// The value passes through argv, which is acceptable here: a local, one-off write
// by the machine's owner, trivial to rotate. It is never written to a file, never
// logged, and never returned to an HTTP client.
func StoreKeychainToken(service, account, token string, timeout time.Duration) error {
	if service == "" || account == "" {
		return fmt.Errorf("keychain service and account are required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is empty")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// -U updates in place when the item already exists, so the call is idempotent.
	//
	// -T whitelists /usr/bin/security as the only reader, which is what makes
	// later headless reads silent. It must NOT be re-sent when the item already
	// exists: changing a Keychain item's ACL is an authorised operation, so
	// rotating a bot token would pop a password dialog ("SecKeychainItemSetAccess:
	// User canceled the operation" when nobody answers). The flag therefore goes
	// on the create path only; updating the value of an existing item is silent.
	args := []string{"add-generic-password", "-U",
		"-s", service, "-a", account, "-w", token}
	if !KeychainTokenReadable(service, account, 5*time.Second) {
		args = append(args, "-T", securityBin)
	}
	args = append(args, keychainArgs()...)
	cmd := exec.CommandContext(ctx, securityBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// A locked login keychain makes security(1) wait for a password that no
		// headless caller can supply, so the context kills it. Say so plainly:
		// this is the one keychain failure a user has to fix by hand.
		if ctx.Err() != nil {
			return fmt.Errorf("keychain is locked — unlock it (Keychain Access, or `security unlock-keychain`) and retry")
		}
		// Redact defensively: the argv we passed includes the token, and some
		// security(1) builds echo usage on failure.
		msg := Redact(token, string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("keychain write failed: %s", strings.TrimSpace(msg))
	}
	return nil
}

// KeychainEnvVar names a keychain file to use instead of the user's default
// login keychain. It exists for tests: `go test` must never write to the login
// keychain, because each test binary is a new process to the item's ACL and
// macOS asks the user for a password every time. Production leaves it unset, so
// the dashboard keeps using the login keychain exactly as documented.
const KeychainEnvVar = "DEPLOYBOARD_KEYCHAIN"

// keychainArgs is the trailing positional keychain argument `security` accepts
// ("If no keychain is specified, the password is added to the default
// keychain"), or nothing when the default (login) keychain should be used.
func keychainArgs() []string {
	if path := strings.TrimSpace(os.Getenv(KeychainEnvVar)); path != "" {
		return []string{path}
	}
	return nil
}

// DeleteKeychainToken removes the item. A missing item is not an error.
func DeleteKeychainToken(service, account string, timeout time.Duration) error {
	if service == "" || account == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	delArgs := []string{"delete-generic-password", "-s", service, "-a", account}
	delArgs = append(delArgs, keychainArgs()...)
	out, err := exec.CommandContext(ctx, securityBin, delArgs...).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "could not be found") {
			return nil
		}
		return fmt.Errorf("keychain delete failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
