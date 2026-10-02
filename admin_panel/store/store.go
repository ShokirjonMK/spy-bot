package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BusinessConfig — bir foydalanuvchining sotuv-agent sozlamalari.
// _id = Telegram user id. business-bot shu hujjatni o'qib javob beradi.
type BusinessConfig struct {
	UserID int64 `bson:"_id" json:"user_id"`

	// Biznes profili
	CompanyName string `bson:"company_name" json:"company_name"`
	ProductInfo string `bson:"product_info" json:"product_info"`
	PriceList   string `bson:"price_list" json:"price_list"`
	FAQ         string `bson:"faq" json:"faq"`

	// Sotuv sozlamasi
	SalesGoal string `bson:"sales_goal" json:"sales_goal"`
	Language  string `bson:"language" json:"language"`
	Tone      string `bson:"tone" json:"tone"`

	// AI backend (mahalliy/boshqa)
	AIEndpoint string `bson:"ai_endpoint" json:"ai_endpoint"`
	AIModel    string `bson:"ai_model" json:"ai_model"`
	AIAPIKey   string `bson:"ai_api_key" json:"ai_api_key"`
	AIFormat   string `bson:"ai_format" json:"ai_format"` // "openai" | "ollama"

	// Javob doirasi
	Enabled       bool    `bson:"enabled" json:"enabled"`               // asosiy kalit
	ReplyAutoNew  bool    `bson:"reply_auto_new" json:"reply_auto_new"` // yangi/notanishga avtomatik
	ReplyWhitelist []int64 `bson:"reply_whitelist" json:"reply_whitelist"`

	BotID     int64     `bson:"bot_id" json:"bot_id"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type Store struct {
	client   *mongo.Client
	configs  *mongo.Collection
	bots     *mongo.Collection
	botUsers *mongo.Collection
	users    *mongo.Collection
	messages *mongo.Collection
	aiChats  *mongo.Collection
	blocked  *mongo.Collection
}

func New(ctx context.Context, uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	db := client.Database(dbName)
	return &Store{
		client:   client,
		configs:  db.Collection("business_config"),
		bots:     db.Collection("bots"),
		botUsers: db.Collection("bot_users"),
		users:    db.Collection("users"),
		messages: db.Collection("telegram_messages_v2"),
		aiChats:  db.Collection("ai_chats"),
		blocked:  db.Collection("blocked_users"),
	}, nil
}

func (s *Store) GetConfig(ctx context.Context, userID int64) (*BusinessConfig, error) {
	var cfg BusinessConfig
	err := s.configs.FindOne(ctx, bson.M{"_id": userID}).Decode(&cfg)
	if err == mongo.ErrNoDocuments {
		// bo'sh, standart config qaytaramiz
		return &BusinessConfig{
			UserID:       userID,
			Language:     "uz",
			Tone:         "samimiy",
			AIFormat:     "openai",
			ReplyAutoNew: true,
			ReplyWhitelist: []int64{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if cfg.ReplyWhitelist == nil {
		cfg.ReplyWhitelist = []int64{}
	}
	return &cfg, nil
}

func (s *Store) SaveConfig(ctx context.Context, cfg *BusinessConfig) error {
	cfg.UpdatedAt = time.Now()
	if cfg.ReplyWhitelist == nil {
		cfg.ReplyWhitelist = []int64{}
	}
	_, err := s.configs.ReplaceOne(
		ctx,
		bson.M{"_id": cfg.UserID},
		cfg,
		options.Replace().SetUpsert(true),
	)
	return err
}

// BotStatus — foydalanuvchining ulangan boti va biznes-ulanishlari holati.
type BotStatus struct {
	HasBot       bool   `json:"has_bot"`
	BotID        int64  `json:"bot_id"`
	BotUsername  string `json:"bot_username"`
	Connections  int    `json:"connections"`
	SendMessages bool   `json:"send_messages"`
}

func (s *Store) Status(ctx context.Context, userID int64) (*BotStatus, error) {
	st := &BotStatus{}

	// bots kolleksiyasi user_id bo'yicha (creator orqali yaratilgan) — bo'lmasa
	// birinchi mavjud botni olamiz (single-bot rejimi uchun).
	var bot struct {
		ID       int64  `bson:"_id"`
		Username string `bson:"username"`
	}
	err := s.bots.FindOne(ctx, bson.M{"user_id": userID}).Decode(&bot)
	if err == mongo.ErrNoDocuments {
		err = s.bots.FindOne(ctx, bson.M{}).Decode(&bot)
	}
	if err == nil {
		st.HasBot = true
		st.BotID = bot.ID
		st.BotUsername = bot.Username
	}

	if st.HasBot {
		var bu struct {
			SendMessages bool  `bson:"send_messages"`
			Conns        []any `bson:"business_connections"`
		}
		e := s.botUsers.FindOne(ctx, bson.M{"user_id": userID, "bot_id": st.BotID}).Decode(&bu)
		if e == nil {
			st.SendMessages = bu.SendMessages
			st.Connections = len(bu.Conns)
		}
	}
	return st, nil
}

// InsertBot — bots kolleksiyasiga bot yozadi (panel orqali ulash).
func (s *Store) InsertBot(ctx context.Context, botID, ownerID int64, token, username string) error {
	_, err := s.bots.ReplaceOne(
		ctx,
		bson.M{"_id": botID},
		bson.M{
			"_id":          botID,
			"username":     username,
			"secret_token": token,
			"user_id":      ownerID,
			"created_at":   time.Now(),
		},
		options.Replace().SetUpsert(true),
	)
	return err
}

// RemoveBot — botni va bog'liq bot_users yozuvlarini o'chiradi.
func (s *Store) RemoveBot(ctx context.Context, botID, ownerID int64) error {
	if _, err := s.bots.DeleteOne(ctx, bson.M{"_id": botID, "user_id": ownerID}); err != nil {
		return err
	}
	_, _ = s.botUsers.DeleteMany(ctx, bson.M{"bot_id": botID, "user_id": ownerID})
	return nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
