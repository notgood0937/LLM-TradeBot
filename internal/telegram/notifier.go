package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"llmtradebot/internal/config"
)

type Notifier struct {
	enabled bool
	token   string
	chatID  string
	client  *http.Client
}

func New(cfg config.TelegramConfig) *Notifier {
	return &Notifier{
		enabled: cfg.Enabled,
		token:   cfg.BotToken,
		chatID:  cfg.ChatID,
		client:  &http.Client{Timeout: cfg.Timeout},
	}
}

func (n *Notifier) Ready() bool {
	return n != nil && n.enabled && n.token != "" && n.chatID != ""
}

func (n *Notifier) Send(ctx context.Context, text string) error {
	if !n.Ready() {
		return nil
	}

	body, _ := json.Marshal(map[string]any{
		"chat_id":                  n.chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send failed: %s", resp.Status)
	}
	return nil
}

func (n *Notifier) SendAsync(text string) {
	if !n.Ready() {
		log.Printf("telegram: send skipped ready=false")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := n.Send(ctx, text); err != nil {
			log.Printf("telegram: send failed err=%v", err)
			return
		}
		log.Printf("telegram: send ok")
	}()
}
