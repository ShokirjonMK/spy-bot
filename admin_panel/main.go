package main

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ssuspy-admin-panel/auth"
	"ssuspy-admin-panel/config"
	"ssuspy-admin-panel/store"
	"ssuspy-admin-panel/tgapi"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	cfg   config.Config
	store *store.Store
}

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	st, err := store.New(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("mongo ulanish xatosi: %v", err)
	}
	log.Printf("MongoDB ulandi: %s", cfg.MongoDB)

	s := &Server{cfg: cfg, store: st}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /app.js", s.handleAsset("web/app.js", "application/javascript"))
	mux.HandleFunc("GET /api/public", s.handlePublic)
	mux.HandleFunc("POST /auth/telegram", s.handleTelegramAuth)
	mux.HandleFunc("GET /auth/dev", s.handleDevAuth)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.withAuth(s.handleMe))
	mux.HandleFunc("GET /api/status", s.withAuth(s.handleStatus))
	mux.HandleFunc("GET /api/config", s.withAuth(s.handleGetConfig))
	mux.HandleFunc("PUT /api/config", s.withAuth(s.handleSaveConfig))
	mux.HandleFunc("POST /api/bot/connect", s.withAuth(s.handleBotConnect))
	mux.HandleFunc("POST /api/bot/disconnect", s.withAuth(s.handleBotDisconnect))
	mux.HandleFunc("GET /api/admin/overview", s.withSuperadmin(s.handleAdminOverview))
	mux.HandleFunc("POST /api/admin/bot/remove", s.withSuperadmin(s.handleAdminBotRemove))
	mux.HandleFunc("POST /api/admin/agent", s.withSuperadmin(s.handleAdminAgent))
	mux.HandleFunc("POST /api/admin/user/block", s.withSuperadmin(s.handleAdminUserBlock))
	mux.HandleFunc("POST /api/admin/user/delete", s.withSuperadmin(s.handleAdminUserDelete))
	mux.HandleFunc("GET /api/admin/user/chats", s.withSuperadmin(s.handleAdminUserChats))
	mux.HandleFunc("GET /api/admin/user/messages", s.withSuperadmin(s.handleAdminUserMessages))

	addr := ":" + cfg.Port
	log.Printf("admin panel ishga tushdi: http://localhost%s (dev=%v)", addr, cfg.DevMode)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	b, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "index topilmadi", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

func (s *Server) handleAsset(path, ctype string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := webFS.ReadFile(path)
		if err != nil {
			http.Error(w, "topilmadi", 404)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Write(b)
	}
}

func (s *Server) handlePublic(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"dev_mode":       s.cfg.DevMode,
		"login_bot":      s.cfg.LoginBotUsername,
		"telegram_ready": s.cfg.AdminBotToken != "" && s.cfg.LoginBotUsername != "",
	})
}

func (s *Server) handleTelegramAuth(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AdminBotToken == "" {
		http.Error(w, "TELEGRAM_TOKEN sozlanmagan", 500)
		return
	}
	r.ParseForm()
	data := map[string]string{}
	for k := range r.Form {
		data[k] = r.Form.Get(k)
	}
	user, err := auth.VerifyTelegramLogin(s.cfg.AdminBotToken, data)
	if err != nil {
		http.Error(w, "login tekshiruvi muvaffaqiyatsiz: "+err.Error(), http.StatusUnauthorized)
		return
	}
	auth.SetSession(w, s.cfg.SessionSecret, user.ID)
	writeJSON(w, map[string]any{"ok": true, "user_id": user.ID})
}

// handleDevAuth — faqat DEV_MODE da: Telegram Login domensiz lokal test uchun.
func (s *Server) handleDevAuth(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevMode {
		http.Error(w, "dev login o'chirilgan", http.StatusForbidden)
		return
	}
	uid, err := strconv.ParseInt(r.URL.Query().Get("uid"), 10, 64)
	if err != nil {
		http.Error(w, "uid noto'g'ri", 400)
		return
	}
	auth.SetSession(w, s.cfg.SessionSecret, uid)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSession(w)
	writeJSON(w, map[string]any{"ok": true})
}

type ctxKey string

const userKey ctxKey = "uid"

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserFromRequest(r, s.cfg.SessionSecret)
		if err != nil {
			http.Error(w, "avtorizatsiya kerak", http.StatusUnauthorized)
			return
		}
		next(w, r, uid)
	}
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, uid int64) {
	writeJSON(w, map[string]any{
		"user_id":     uid,
		"dev_mode":    s.cfg.DevMode,
		"superadmin":  s.cfg.IsSuperadmin(uid),
	})
}

func (s *Server) withSuperadmin(next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserFromRequest(r, s.cfg.SessionSecret)
		if err != nil {
			http.Error(w, "avtorizatsiya kerak", http.StatusUnauthorized)
			return
		}
		if !s.cfg.IsSuperadmin(uid) {
			http.Error(w, "faqat superadmin uchun", http.StatusForbidden)
			return
		}
		next(w, r, uid)
	}
}

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request, uid int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	stats, err := s.store.AdminStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	users, _ := s.store.AdminUsers(ctx)
	bots, _ := s.store.AdminBots(ctx)
	writeJSON(w, map[string]any{"stats": stats, "users": users, "bots": bots})
}

func (s *Server) handleAdminBotRemove(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		ID int64 `json:"id"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if json.Unmarshal(body, &req) != nil || req.ID == 0 {
		http.Error(w, "id kerak", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	_ = tgapi.CallBusiness(ctx, s.cfg.BusinessInternalURL, s.cfg.InternalSecret, "remove", req.ID)
	if err := s.store.AdminRemoveBot(ctx, req.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminAgent(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		OwnerID int64 `json:"owner_id"`
		Enabled bool  `json:"enabled"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if json.Unmarshal(body, &req) != nil || req.OwnerID == 0 {
		http.Error(w, "owner_id kerak", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.store.AdminSetAgent(ctx, req.OwnerID, req.Enabled); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, uid int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	st, err := s.store.Status(ctx, uid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, st)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request, uid int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cfg, err := s.store.GetConfig(ctx, uid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, cfg)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request, uid int64) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var cfg store.BusinessConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		http.Error(w, "json xato: "+err.Error(), 400)
		return
	}
	cfg.UserID = uid // sessiyadan, mijozdan emas

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// bot_id ni status'dan to'ldiramiz (agar bo'sh bo'lsa)
	if cfg.BotID == 0 {
		if st, err := s.store.Status(ctx, uid); err == nil && st.HasBot {
			cfg.BotID = st.BotID
		}
	}

	if err := s.store.SaveConfig(ctx, &cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleBotConnect(w http.ResponseWriter, r *http.Request, uid int64) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Token) == "" {
		http.Error(w, "token kerak", 400)
		return
	}
	req.Token = strings.TrimSpace(req.Token)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if s.store.IsBlocked(ctx, uid) {
		http.Error(w, "hisobingiz bloklangan", http.StatusForbidden)
		return
	}

	info, err := tgapi.GetMe(ctx, req.Token)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !info.CanConnectBusiness {
		http.Error(w, "Bu botda Business Mode o'chiq. @BotFather → Bot Settings → Business Mode → Turn on qiling, so'ng qayta urining.", 400)
		return
	}

	if err := s.store.InsertBot(ctx, info.ID, uid, req.Token, info.Username); err != nil {
		http.Error(w, "DB xato: "+err.Error(), 500)
		return
	}

	if err := tgapi.CallBusiness(ctx, s.cfg.BusinessInternalURL, s.cfg.InternalSecret, "add", info.ID); err != nil {
		http.Error(w, "bot ishga tushmadi: "+err.Error(), 500)
		return
	}

	writeJSON(w, map[string]any{"ok": true, "bot_id": info.ID, "username": info.Username})
}

func (s *Server) handleBotDisconnect(w http.ResponseWriter, r *http.Request, uid int64) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	st, err := s.store.Status(ctx, uid)
	if err != nil || !st.HasBot {
		http.Error(w, "ulangan bot topilmadi", 404)
		return
	}

	_ = tgapi.CallBusiness(ctx, s.cfg.BusinessInternalURL, s.cfg.InternalSecret, "remove", st.BotID)

	if err := s.store.RemoveBot(ctx, st.BotID, uid); err != nil {
		http.Error(w, "o'chirishda xato: "+err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminUserBlock(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		OwnerID int64 `json:"owner_id"`
		Blocked bool  `json:"blocked"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if json.Unmarshal(body, &req) != nil || req.OwnerID == 0 {
		http.Error(w, "owner_id kerak", 400)
		return
	}
	if s.cfg.IsSuperadmin(req.OwnerID) {
		http.Error(w, "superadminni bloklab bo'lmaydi", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if err := s.store.AdminSetBlocked(ctx, req.OwnerID, req.Blocked); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// bloklansa — botlarini to'xtatamiz
	if req.Blocked {
		if ids, e := s.store.UserBotIDs(ctx, req.OwnerID); e == nil {
			for _, id := range ids {
				_ = tgapi.CallBusiness(ctx, s.cfg.BusinessInternalURL, s.cfg.InternalSecret, "remove", id)
			}
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminUserDelete(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		OwnerID int64 `json:"owner_id"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if json.Unmarshal(body, &req) != nil || req.OwnerID == 0 {
		http.Error(w, "owner_id kerak", 400)
		return
	}
	if s.cfg.IsSuperadmin(req.OwnerID) {
		http.Error(w, "superadminni o'chirib bo'lmaydi", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// botlarini to'xtatamiz
	if ids, e := s.store.UserBotIDs(ctx, req.OwnerID); e == nil {
		for _, id := range ids {
			_ = tgapi.CallBusiness(ctx, s.cfg.BusinessInternalURL, s.cfg.InternalSecret, "remove", id)
		}
	}
	if err := s.store.AdminDeleteUser(ctx, req.OwnerID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminUserChats(w http.ResponseWriter, r *http.Request, uid int64) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if id == 0 {
		http.Error(w, "id kerak", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	chats, err := s.store.AdminUserChats(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, chats)
}

func (s *Server) handleAdminUserMessages(w http.ResponseWriter, r *http.Request, uid int64) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	chat, _ := strconv.ParseInt(r.URL.Query().Get("chat"), 10, 64)
	if id == 0 || chat == 0 {
		http.Error(w, "id va chat kerak", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	msgs, err := s.store.AdminChatMessages(ctx, id, chat)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, msgs)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
