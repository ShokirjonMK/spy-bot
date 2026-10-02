package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BusinessConfig — admin panel orqali kiritilgan sotuv-agent sozlamasi.
// _id = egasi (owner) Telegram id.
type BusinessConfig struct {
	UserID int64 `bson:"_id"`

	CompanyName string `bson:"company_name"`
	ProductInfo string `bson:"product_info"`
	PriceList   string `bson:"price_list"`
	FAQ         string `bson:"faq"`

	SalesGoal string `bson:"sales_goal"`
	Language  string `bson:"language"`
	Tone      string `bson:"tone"`

	AIEndpoint string `bson:"ai_endpoint"`
	AIModel    string `bson:"ai_model"`
	AIAPIKey   string `bson:"ai_api_key"`
	AIFormat   string `bson:"ai_format"`

	Enabled        bool    `bson:"enabled"`
	ReplyAutoNew   bool    `bson:"reply_auto_new"`
	ReplyWhitelist []int64 `bson:"reply_whitelist"`

	BotID int64 `bson:"bot_id"`
}

func (r *MongoRepository) GetBusinessConfig(ctx context.Context, ownerID int64) (*BusinessConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cfg BusinessConfig
	err := r.businessConfig.FindOne(ctx, bson.M{"_id": ownerID}).Decode(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// --- ai_chats: per-chat dinamik holat ---

type AIChat struct {
	OwnerID int64 `bson:"owner_id"`
	BotID   int64 `bson:"bot_id"`
	ChatID  int64 `bson:"chat_id"`
	Active  bool  `bson:"active"`
}

// GetAIChat — chat holatini qaytaradi. found=false bo'lsa hali qaror qilinmagan (yangi chat).
func (r *MongoRepository) GetAIChat(ctx context.Context, ownerID, botID, chatID int64) (active bool, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var ac AIChat
	err = r.aiChats.FindOne(ctx, bson.M{"owner_id": ownerID, "bot_id": botID, "chat_id": chatID}).Decode(&ac)
	if err == mongo.ErrNoDocuments {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return ac.Active, true, nil
}

func (r *MongoRepository) SetAIChat(ctx context.Context, ownerID, botID, chatID int64, active bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.aiChats.UpdateOne(
		ctx,
		bson.M{"owner_id": ownerID, "bot_id": botID, "chat_id": chatID},
		bson.M{"$set": bson.M{"active": active, "updated_at": time.Now()}},
		options.Update().SetUpsert(true),
	)
	return err
}

// RecentChatMessage — AI kontekst uchun yengil ko'rinish (custom registry'siz).
type RecentChatMessage struct {
	FromID    int64  `bson:"from_id"`
	Text      string `bson:"text"`
	Caption   string `bson:"caption"`
	MessageID int    `bson:"mid"`
}

// RecentChatMessages — chatdagi oxirgi N ta xabar (eskidan yangiga tartiblangan).
func (r *MongoRepository) RecentChatMessages(ctx context.Context, chatID int64, connectionIDs []string, limit int) ([]RecentChatMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "message.chat.id", Value: chatID},
			{Key: "message.business_connection_id", Value: bson.D{{Key: "$in", Value: connectionIDs}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "message.message_id", Value: -1}, {Key: "message.edit_date", Value: -1}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$message.message_id"},
			{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: -1}}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "from_id", Value: "$doc.message.from.id"},
			{Key: "text", Value: "$doc.message.text"},
			{Key: "caption", Value: "$doc.message.caption"},
			{Key: "mid", Value: "$doc.message.message_id"},
		}}},
	}

	cursor, err := r.telegramMessages.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []RecentChatMessage
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}

	// eskidan yangiga
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
