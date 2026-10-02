package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const cookieName = "ap_session"

// TelegramUser — Login Widget dan kelgan ma'lumot.
type TelegramUser struct {
	ID        int64
	FirstName string
	Username  string
	PhotoURL  string
	AuthDate  int64
}

// VerifyTelegramLogin — Telegram Login Widget hash'ini tekshiradi.
// Kalit = SHA256(bot_token), keyin data-check-string ustidan HMAC-SHA256.
// https://core.telegram.org/widgets/login#checking-authorization
func VerifyTelegramLogin(botToken string, data map[string]string) (*TelegramUser, error) {
	hash := data["hash"]
	if hash == "" {
		return nil, errors.New("hash yo'q")
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, data[k]))
	}
	dataCheckString := strings.Join(parts, "\n")

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return nil, errors.New("hash mos kelmadi")
	}

	authDate, _ := strconv.ParseInt(data["auth_date"], 10, 64)
	if time.Now().Unix()-authDate > 86400 {
		return nil, errors.New("login eskirgan")
	}

	id, err := strconv.ParseInt(data["id"], 10, 64)
	if err != nil {
		return nil, errors.New("id noto'g'ri")
	}

	return &TelegramUser{
		ID:        id,
		FirstName: data["first_name"],
		Username:  data["username"],
		PhotoURL:  data["photo_url"],
		AuthDate:  authDate,
	}, nil
}

// --- Sessiya (imzolangan cookie, tashqi kutubxonasiz) ---

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// SetSession — userID uchun imzolangan cookie o'rnatadi (7 kun).
func SetSession(w http.ResponseWriter, secret string, userID int64) {
	exp := time.Now().Add(7 * 24 * time.Hour).Unix()
	payload := fmt.Sprintf("%d:%d", userID, exp)
	value := payload + ":" + sign(secret, payload)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(exp, 0),
	})
}

func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// UserFromRequest — cookie'dan userID ni tekshirib qaytaradi.
func UserFromRequest(r *http.Request, secret string) (int64, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return 0, errors.New("sessiya yo'q")
	}
	parts := strings.Split(c.Value, ":")
	if len(parts) != 3 {
		return 0, errors.New("cookie buzuq")
	}
	payload := parts[0] + ":" + parts[1]
	if !hmac.Equal([]byte(sign(secret, payload)), []byte(parts[2])) {
		return 0, errors.New("imzo noto'g'ri")
	}
	exp, _ := strconv.ParseInt(parts[1], 10, 64)
	if time.Now().Unix() > exp {
		return 0, errors.New("sessiya muddati tugagan")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
