// Package simplefin is a minimal SimpleFIN Bridge client.
// Protocol: setup token (base64 claim URL) → POST → Access URL whose embedded
// basic-auth userinfo is the only credential. https://www.simplefin.org/protocol.html
package simplefin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Org struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type Transaction struct {
	ID          string `json:"id"`
	Posted      int64  `json:"posted"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Pending     bool   `json:"pending"`
}

func (t Transaction) PostedTime() time.Time { return time.Unix(t.Posted, 0).UTC() }

type Account struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Currency     string        `json:"currency"`
	Balance      string        `json:"balance"`
	BalanceDate  int64         `json:"balance-date"`
	Org          Org           `json:"org"`
	Transactions []Transaction `json:"transactions"`
}

func (a Account) BalanceTime() time.Time { return time.Unix(a.BalanceDate, 0).UTC() }

type AccountSet struct {
	Errors   []string  `json:"errors"`
	Accounts []Account `json:"accounts"`
}

// Claim exchanges a one-time setup token for the Access URL. Call once, save
// the result to 1Password, and never write it to disk.
func Claim(setupToken string) (string, error) {
	claimURL, err := base64.StdEncoding.DecodeString(strings.TrimSpace(setupToken))
	if err != nil {
		return "", fmt.Errorf("setup token is not base64: %w", err)
	}
	resp, err := http.Post(string(claimURL), "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claim failed: %s: %s", resp.Status, body)
	}
	return strings.TrimSpace(string(body)), nil
}

type Client struct {
	user, pass, host, base string
	scheme                 string
	hc                     *http.Client
}

// New parses an Access URL of the form https://user:pass@host/path.
func New(accessURL string) *Client {
	u, err := url.Parse(accessURL)
	if err != nil || u.User == nil {
		// Fail loudly on first request rather than panicking at startup.
		return &Client{scheme: "https", hc: http.DefaultClient}
	}
	pass, _ := u.User.Password()
	return &Client{
		user: u.User.Username(), pass: pass,
		host: u.Host, base: strings.TrimSuffix(u.Path, "/"),
		scheme: "https",
		hc:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Accounts fetches all accounts with transactions posted on/after start.
// SimpleFIN allows at most a 90-day window per request; callers (sync) stay
// far inside that. pending=true includes pending transactions.
func (c *Client) Accounts(ctx context.Context, start time.Time, pending bool) (*AccountSet, error) {
	if c.host == "" {
		return nil, fmt.Errorf("invalid SimpleFIN access URL (missing credentials or host)")
	}
	q := url.Values{"start-date": {fmt.Sprint(start.Unix())}}
	if pending {
		q.Set("pending", "1")
	}
	reqURL := fmt.Sprintf("%s://%s%s/accounts?%s", c.scheme, c.host, c.base, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.pass)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("simplefin /accounts: %s: %s", resp.Status, body)
	}
	var set AccountSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode /accounts: %w", err)
	}
	return &set, nil
}
