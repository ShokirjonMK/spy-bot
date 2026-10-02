package telegram

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"ssuspy-bot/config"
	"ssuspy-bot/repository"
	"ssuspy-bot/telegram/manager"
)

// registerInternalAPI — admin panel chaqiradigan ichki API (botlarni ulash/uzish).
// Faqat ichki tarmoqda, X-Internal-Secret sarlavhasi bilan himoyalangan.
func registerInternalAPI(mux *http.ServeMux, mng *manager.BotManager, mongo *repository.MongoRepository) {
	secret := config.Config.InternalSecret

	auth := func(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if secret == "" || r.Header.Get("X-Internal-Secret") != secret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	}

	type botReq struct {
		ID int64 `json:"id"`
	}

	mux.HandleFunc("POST /internal/bot/add", auth(func(w http.ResponseWriter, r *http.Request) {
		var req botReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
			http.Error(w, "id kerak", http.StatusBadRequest)
			return
		}

		botData, err := mongo.BotByID(r.Context(), req.ID)
		if err != nil {
			http.Error(w, "bot DB'da topilmadi", http.StatusNotFound)
			return
		}

		if err := mng.AddBot(r.Context(), req.ID, botData.SecretToken); err != nil {
			// allaqachon ishlayotgan bo'lsa — muvaffaqiyat deb hisoblaymiz (idempotent)
			if _, exists := mng.GetBot(req.ID); exists {
				writeJSON(w, map[string]any{"ok": true, "already": true})
				return
			}
			log.Error().Err(err).Int64("botID", req.ID).Msg("internal add bot failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}))

	mux.HandleFunc("POST /internal/bot/remove", auth(func(w http.ResponseWriter, r *http.Request) {
		var req botReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
			http.Error(w, "id kerak", http.StatusBadRequest)
			return
		}
		if err := mng.RemoveBot(req.ID); err != nil {
			// allaqachon o'chirilgan bo'lsa — ok
			if _, exists := mng.GetBot(req.ID); !exists {
				writeJSON(w, map[string]any{"ok": true, "already": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}))

	log.Info().Msg("internal API registered (/internal/bot/*)")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
