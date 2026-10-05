package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ErrNoToken means the Keychain item for the bot token is missing/unreadable.
var ErrNoToken = errors.New("telegram bot token not found in keychain")

// securityBin is the stable, ACL-whitelisted reader: pinning the ACL to this
// system binary keeps headless reads working across toolchain upgrades.
const securityBin = "/usr/bin/security"

// ReadKeychainToken reads a generic-password secret from the macOS Keychain.
// The value is returned to the caller only — it is never logged.
func ReadKeychainToken(service, account string, timeout time.Duration) (string, error) {
	if service == "" || account == "" {
		return "", ErrNoToken
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// The `timeout` shell binary does not exist on macOS; the context is the
	// guard, exactly as the secrets-vault skill records.
	findArgs := []string{"find-generic-password", "-s", service, "-a", account, "-w"}
	findArgs = append(findArgs, keychainArgs()...)
	out, err := exec.CommandContext(ctx, securityBin, findArgs...).Output()
	if err != nil {
		return "", fmt.Errorf("%w (service=%s account=%s)", ErrNoToken, service, account)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", ErrNoToken
	}
	return token, nil
}

// KeychainTokenReadable reports whether the token can be read, without exposing it.
func KeychainTokenReadable(service, account string, timeout time.Duration) bool {
	_, err := ReadKeychainToken(service, account, timeout)
	return err == nil
}

// Redact removes a secret from a string so it can be logged safely.
func Redact(secret, s string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}

// messagePayload is the Telegram sendMessage body. Deliberately no parse_mode:
// launchd labels and log paths contain characters that break Markdown/HTML
// parsing and would make the alert fail to deliver.
type messagePayload struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	DisableNotification bool   `json:"disable_notification"`
}

// Sender posts messages to the Telegram Bot API.
type Sender struct {
	Token  string
	ChatID string
	Client *http.Client
	// BaseURL is overridable in tests; defaults to the public Bot API.
	BaseURL string
}

// NewSender builds a Sender, defaulting the HTTP client and base URL.
func NewSender(token, chatID string, timeout time.Duration) *Sender {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Sender{
		Token:   token,
		ChatID:  chatID,
		Client:  &http.Client{Timeout: timeout},
		BaseURL: "https://api.telegram.org",
	}
}

// Send posts one message, returning the HTTP status. Errors never contain the
// token or the full request URL (only the status code and Telegram's description).
func (s *Sender) Send(ctx context.Context, text string) (int, error) {
	if s == nil || s.Token == "" {
		return 0, ErrNoToken
	}
	if s.ChatID == "" {
		return 0, errors.New("telegram chat_id is not configured")
	}
	body, err := json.Marshal(messagePayload{ChatID: s.ChatID, Text: text, DisableNotification: false})
	if err != nil {
		return 0, err
	}
	url := strings.TrimRight(s.BaseURL, "/") + "/bot" + s.Token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		// Redact defensively: net/http errors can embed the request URL.
		return 0, errors.New(Redact(s.Token, err.Error()))
	}
	defer resp.Body.Close()

	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if resp.StatusCode != http.StatusOK || !parsed.OK {
		desc := parsed.Description
		if desc == "" {
			desc = http.StatusText(resp.StatusCode)
		}
		return resp.StatusCode, fmt.Errorf("telegram sendMessage failed: status=%d %s", resp.StatusCode, desc)
	}
	return resp.StatusCode, nil
}
