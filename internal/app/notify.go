package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var forbiddenNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
}

func publicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range forbiddenNetworks {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func webhookClient() *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("webhook DNS lookup failed")
			}
			for _, ip := range ips {
				if !publicAddress(ip) {
					return nil, errors.New("webhook resolves to a non-public address")
				}
			}
			dialer := net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("webhook redirects disabled") }}
}

func ValidateWebhook(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return errors.New("webhook must be a public HTTPS URL on port 443 without userinfo or fragment")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicAddress(ip) {
		return errors.New("webhook address must be public")
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		return errors.New("webhook address must be public")
	}
	return nil
}

func (w *Worker) notifyPending(ctx context.Context) error {
	var id, name, status string
	var attempts int
	err := w.Store.DB.QueryRowContext(ctx, `SELECT id,project_name,status,notification_attempts FROM jobs WHERE notification_status='pending' AND notification_next_at<=clock_timestamp() ORDER BY notification_next_at LIMIT 1`).Scan(&id, &name, &status, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if w.WebhookURL == "" {
		_, err = w.Store.DB.ExecContext(ctx, "UPDATE jobs SET notification_status='skipped' WHERE id=$1", id)
		return err
	}
	payload, _ := json.Marshal(map[string]any{"event": "restivra.verification.failed", "job_id": id, "project": name, "status": status, "text": "Restivra: " + name + " / " + status + " / " + id})
	deliverErr := ValidateWebhook(w.WebhookURL)
	if deliverErr == nil {
		req, e := http.NewRequestWithContext(ctx, http.MethodPost, w.WebhookURL, bytes.NewReader(payload))
		deliverErr = e
		if e == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Restivra/0.1")
			client := webhookClient()
			resp, e := client.Do(req)
			deliverErr = e
			if e == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					deliverErr = errors.New("webhook rejected")
				}
			}
			client.CloseIdleConnections()
		}
	}
	state := "delivered"
	attempts++
	if deliverErr != nil {
		state = "pending"
		if attempts >= 3 {
			state = "failed"
		}
	}
	// Verification status is deliberately untouched by notification delivery.
	_, err = w.Store.DB.ExecContext(ctx, `UPDATE jobs SET notification_status=$2,notification_attempts=$3,notification_next_at=clock_timestamp()+($4::integer * interval '1 second') WHERE id=$1`, id, state, attempts, attempts*60)
	return err
}
