package tgapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BotInfo struct {
	ID                  int64  `json:"id"`
	Username            string `json:"username"`
	CanConnectBusiness  bool   `json:"can_connect_to_business"`
	SupportsInline      bool   `json:"supports_inline_queries"`
}

// GetMe — rasmiy Telegram API orqali bot tokenini tekshiradi.
func GetMe(ctx context.Context, token string) (*BotInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var out struct {
		OK          bool    `json:"ok"`
		Description string  `json:"description"`
		Result      BotInfo `json:"result"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("javob o'qib bo'lmadi")
	}
	if !out.OK {
		return nil, fmt.Errorf("token yaroqsiz: %s", out.Description)
	}
	return &out.Result, nil
}

// CallBusiness — business-bot ichki API'siga so'rov (add|remove).
func CallBusiness(ctx context.Context, baseURL, secret, action string, botID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]int64{"id": botID})
	url := baseURL + "/internal/bot/" + action
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		d, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("business-bot xato (%d): %s", resp.StatusCode, string(d))
	}
	return nil
}
