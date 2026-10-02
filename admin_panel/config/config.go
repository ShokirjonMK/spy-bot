package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func parseIDs(s string) []int64 {
	var out []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

type Config struct {
	MongoURI      string
	MongoDB       string
	AdminBotToken    string // login bot token, used to verify Telegram Login hash
	LoginBotUsername string // login widget uchun bot username (@ siz)
	SessionSecret    string
	Port             string
	DevMode          bool

	BusinessInternalURL string  // business-bot ichki API (http://business-bot:8080)
	InternalSecret      string  // ichki API siri
	SuperadminIDs       []int64 // to'liq boshqaruvga ega adminlar
}

func (c Config) IsSuperadmin(uid int64) bool {
	for _, id := range c.SuperadminIDs {
		if id == uid {
			return true
		}
	}
	return false
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	host := envDefault("MONGO_HOST", "mongodb")
	port := envDefault("MONGO_PORT", "27017")
	db := envDefault("MONGO_DB", "ssuspy")
	user := os.Getenv("MONGO_USERNAME")
	pass := os.Getenv("MONGO_PASSWORD")

	auth := ""
	if user != "" && pass != "" {
		auth = fmt.Sprintf("%s:%s@", url.QueryEscape(user), url.QueryEscape(pass))
	}
	uri := fmt.Sprintf("mongodb://%s%s:%s/%s", auth, host, port, db)

	dev, _ := strconv.ParseBool(envDefault("DEV_MODE", "false"))

	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "dev-insecure-secret-change-me"
	}

	return Config{
		MongoURI:      uri,
		MongoDB:       db,
		AdminBotToken:    os.Getenv("TELEGRAM_TOKEN"),
		LoginBotUsername: envDefault("LOGIN_BOT_USERNAME", ""),
		SessionSecret:    secret,
		Port:             envDefault("PORT", "8090"),
		DevMode:          dev,

		BusinessInternalURL: envDefault("BUSINESS_INTERNAL_URL", "http://business-bot:8080"),
		InternalSecret:      os.Getenv("INTERNAL_SECRET"),
		SuperadminIDs:       parseIDs(os.Getenv("SUPERADMIN_IDS")),
	}
}
