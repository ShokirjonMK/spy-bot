package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ssuspy-bot/repository"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildSystemPrompt — biznes config asosida sotuv-agent uchun system prompt.
func BuildSystemPrompt(cfg *repository.BusinessConfig) string {
	var b strings.Builder
	company := cfg.CompanyName
	if company == "" {
		company = "kompaniya"
	}
	b.WriteString(fmt.Sprintf("Sen %s uchun ishlaydigan professional sotuv-maslahatchisisan. ", company))
	b.WriteString("Sen mijoz bilan Telegram orqali uning nomidan yozishayapsan. ")

	switch cfg.Language {
	case "ru":
		b.WriteString("Отвечай на русском языке. ")
	case "en":
		b.WriteString("Reply in English. ")
	case "auto":
		b.WriteString("Suhbatdosh qaysi tilda yozsa, o'sha tilda javob ber. ")
	default:
		b.WriteString("O'zbek tilida javob ber. ")
	}

	switch cfg.Tone {
	case "rasmiy":
		b.WriteString("Uslub: rasmiy va hurmatli. ")
	case "qiziqarli":
		b.WriteString("Uslub: qiziqarli va jonli. ")
	default:
		b.WriteString("Uslub: samimiy va do'stona. ")
	}

	switch cfg.SalesGoal {
	case "order":
		b.WriteString("Maqsad: mijozdan buyurtma olish. Buyurtma tafsilotlarini (miqdor, manzil, aloqa) aniqlashtir. ")
	case "call":
		b.WriteString("Maqsad: mijozni qo'ng'iroqqa yozib qo'yish. Uning telefon raqami va qulay vaqtini so'ra. ")
	case "inform":
		b.WriteString("Maqsad: mahsulot haqida ma'lumot berish va qiziqtirish. ")
	default:
		b.WriteString("Maqsad: mijozning aloqa ma'lumotini (ism, telefon) olish va qiziqishini aniqlash. ")
	}

	if cfg.ProductInfo != "" {
		b.WriteString("\n\nMAHSULOT/XIZMAT:\n" + cfg.ProductInfo)
	}
	if cfg.PriceList != "" {
		b.WriteString("\n\nNARXLAR:\n" + cfg.PriceList)
	}
	if cfg.FAQ != "" {
		b.WriteString("\n\nTEZ-TEZ SO'RALADIGAN SAVOLLAR:\n" + cfg.FAQ)
	}

	b.WriteString("\n\nQOIDALAR: Qisqa va aniq yoz (1-3 gap). Faqat mahsulot/xizmat va sotuv mavzusida gaplash. ")
	b.WriteString("Narxni so'rasa — narxlar ro'yxatidan ayt. Bilmagan narsangni o'ylab topma, 'aniqlashtirib aytaman' de. ")
	b.WriteString("Hech qachon sun'iy intellekt yoki bot ekaningni aytma.")

	return b.String()
}

// BuildHistory — saqlangan xabarlardan AI uchun tarix (owner=assistant, partner=user).
func BuildHistory(msgs []repository.RecentChatMessage, ownerID int64) []Message {
	var out []Message
	for _, m := range msgs {
		text := m.Text
		if text == "" {
			text = m.Caption
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		role := "user"
		if m.FromID == ownerID {
			role = "assistant"
		}
		out = append(out, Message{Role: role, Content: text})
	}
	return out
}

// Generate — configdagi endpointga so'rov yuborib javob matnini qaytaradi.
func Generate(ctx context.Context, cfg *repository.BusinessConfig, history []Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	system := BuildSystemPrompt(cfg)
	messages := append([]Message{{Role: "system", Content: system}}, history...)

	if cfg.AIFormat == "ollama" {
		return generateOllama(ctx, cfg, messages)
	}
	return generateOpenAI(ctx, cfg, messages)
}

func generateOpenAI(ctx context.Context, cfg *repository.BusinessConfig, messages []Message) (string, error) {
	body := map[string]any{
		"model":       cfg.AIModel,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  500,
		"stream":      false,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.AIEndpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.AIAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AIAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai status %d: %s", resp.StatusCode, string(data))
	}

	var out struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("ai json: %w; body=%s", err, string(data))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("ai: bo'sh javob: %s", string(data))
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func generateOllama(ctx context.Context, cfg *repository.BusinessConfig, messages []Message) (string, error) {
	body := map[string]any{
		"model":    cfg.AIModel,
		"messages": messages,
		"stream":   false,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.AIEndpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.AIAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AIAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai status %d: %s", resp.StatusCode, string(data))
	}

	var out struct {
		Message Message `json:"message"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("ai json: %w; body=%s", err, string(data))
	}
	return strings.TrimSpace(out.Message.Content), nil
}
