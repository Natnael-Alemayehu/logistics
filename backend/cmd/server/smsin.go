package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/config"
)

// smsIn hands the running server a text message as if a phone had sent it
// to the SMS number, to try the driver SMS commands from a terminal:
//
//	kiflet sms-in 0912345601 "1 4F2K"
//
// It goes through the inbound webhook, the way the provider delivers texts.
func smsIn(ctx context.Context, cfg *config.Config, args []string) error {
	if len(args) < 2 {
		return errors.New(`usage: kiflet sms-in <phone> <text>, for example: kiflet sms-in 0912345601 "1 4F2K"`)
	}
	payload, _ := json.Marshal(map[string]string{"from": args[0], "body": strings.Join(args[1:], " ")})
	url := "http://localhost:" + cfg.Port + "/api/v1/sms/inbound"
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.SMSInboundSecret != "" {
		req.Header.Set("X-Webhook-Secret", cfg.SMSInboundSecret)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the server on port %s (is it running?): %w", cfg.Port, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Outcome string `json:"outcome"`
		Detail  string `json:"detail"`
		Reply   string `json:"reply"`
	}
	if json.Unmarshal(body, &out) == nil && out.Outcome != "" {
		fmt.Printf("%s: %s\n", out.Outcome, out.Detail)
		if out.Reply != "" {
			fmt.Printf("Reply: %s\n", out.Reply)
		}
		return nil
	}
	fmt.Println("Delivered. Any reply is printed in the server's terminal.")
	return nil
}
