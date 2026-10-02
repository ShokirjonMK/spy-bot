package handlers

import (
	"context"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/rs/zerolog/log"

	"ssuspy-bot/ai"
	"ssuspy-bot/repository"
)

var aiCmdRe = regexp.MustCompile(`(?i)^\s*\.ai\s+(on|off|yoq|ochir)\b`)

// handleAISales — partner xabariga sotuv-agent javobini generatsiya qilib yuboradi.
// HandleMessage ichidan chaqiriladi (xabar saqlangandan keyin).
func (h *Handler) handleAISales(c *th.Context, message *telego.Message) {
	botID, _ := c.Value("botID").(int64)
	iUser, _ := c.Value("iUser").(*repository.IUser)
	if iUser == nil {
		return
	}
	ownerID := iUser.User.ID
	chatID := message.Chat.ID
	connID := message.BusinessConnectionID

	// Egasining o'z xabari — AI javob bermaydi (buyruqlar alohida handlerda).
	if message.From != nil && message.From.ID == ownerID {
		return
	}

	ctx := context.Background()

	cfg, err := h.service.GetBusinessConfig(ctx, ownerID)
	if err != nil || cfg == nil || !cfg.Enabled {
		return
	}
	if strings.TrimSpace(cfg.AIEndpoint) == "" || strings.TrimSpace(cfg.AIModel) == "" {
		return
	}

	// Chat holati (dinamik): mavjud bo'lsa uni, bo'lmasa avto/whitelist bo'yicha qaror.
	active, found, err := h.service.GetAIChat(ctx, ownerID, botID, chatID)
	if err != nil {
		log.Warn().Err(err).Msg("ai: GetAIChat xatosi")
		return
	}
	if !found {
		active = cfg.ReplyAutoNew || inList(cfg.ReplyWhitelist, chatID)
		if e := h.service.SetAIChat(ctx, ownerID, botID, chatID, active); e != nil {
			log.Warn().Err(e).Msg("ai: SetAIChat xatosi")
		}
	}
	if !active {
		return
	}

	// Suhbat tarixi (oxirgi 12 xabar)
	history, err := h.service.RecentChatMessages(ctx, chatID, iUser.BotUser.GetUserCurrentConnectionIDs(), 12)
	if err != nil {
		log.Warn().Err(err).Msg("ai: RecentChatMessages xatosi")
	}
	msgs := ai.BuildHistory(history, ownerID)
	if len(msgs) == 0 {
		// tarix bo'sh bo'lsa joriy xabarni ishlatamiz
		text := message.Text
		if text == "" {
			text = message.Caption
		}
		if strings.TrimSpace(text) == "" {
			return
		}
		msgs = []ai.Message{{Role: "user", Content: text}}
	}

	// "yozmoqda..." holati
	_ = c.Bot().SendChatAction(ctx, &telego.SendChatActionParams{
		BusinessConnectionID: connID,
		ChatID:               tu.ID(chatID),
		Action:               telego.ChatActionTyping,
	})

	reply, err := ai.Generate(ctx, cfg, msgs)
	if err != nil {
		log.Warn().Err(err).Int64("chatID", chatID).Msg("ai: generatsiya xatosi")
		return
	}
	if strings.TrimSpace(reply) == "" {
		return
	}

	sent, err := c.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), reply).
		WithBusinessConnectionID(connID))
	if err != nil {
		log.Warn().Err(err).Int64("chatID", chatID).Msg("ai: javob yuborish xatosi")
		return
	}

	// Botning o'z javobini ham ignore qilamiz (o'chirilgan/kuzatuv logiga tushmasin).
	if sent != nil {
		_ = h.rdb.IgnoreMessage(ctx, sent.MessageID, chatID)
	}
}

// HandleUserAI — egasi ".ai on|off" buyrug'i bilan chatda AI ni yoqadi/o'chiradi.
func (h *Handler) HandleUserAI(c *th.Context, update telego.Update) error {
	message := update.BusinessMessage
	iUser, _ := c.Value("iUser").(*repository.IUser)
	botID, _ := c.Value("botID").(int64)
	if iUser == nil {
		return nil
	}

	m := aiCmdRe.FindStringSubmatch(message.Text)
	if m == nil {
		return nil
	}
	on := false
	switch strings.ToLower(m[1]) {
	case "on", "yoq":
		on = true
	}

	ctx := context.Background()
	if err := h.service.SetAIChat(ctx, iUser.User.ID, botID, message.Chat.ID, on); err != nil {
		log.Warn().Err(err).Msg("ai: SetAIChat (buyruq) xatosi")
		return nil
	}

	txt := "🤖 AI sotuv-agent bu chatda <b>o'chirildi</b>."
	if on {
		txt = "🤖 AI sotuv-agent bu chatda <b>yoqildi</b>."
	}
	_, err := c.Bot().SendMessage(ctx, tu.Message(tu.ID(iUser.User.ID), txt).WithParseMode(telego.ModeHTML))
	return err
}

func inList(list []int64, v int64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
