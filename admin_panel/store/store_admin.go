package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AdminStats struct {
	Users        int64 `json:"users"`
	Bots         int64 `json:"bots"`
	Connections  int64 `json:"connections"`
	Messages     int64 `json:"messages"`
	ActiveAgents int64 `json:"active_agents"`
	AIChats      int64 `json:"ai_chats"`
}

func (s *Store) AdminStats(ctx context.Context) (*AdminStats, error) {
	st := &AdminStats{}
	st.Users, _ = s.users.CountDocuments(ctx, bson.M{})
	st.Bots, _ = s.bots.CountDocuments(ctx, bson.M{})
	st.Messages, _ = s.messages.CountDocuments(ctx, bson.M{})
	st.ActiveAgents, _ = s.configs.CountDocuments(ctx, bson.M{"enabled": true})
	st.AIChats, _ = s.aiChats.CountDocuments(ctx, bson.M{"active": true})

	// enabled biznes-ulanishlar soni
	cur, err := s.botUsers.Aggregate(ctx, []bson.M{
		{"$unwind": "$business_connections"},
		{"$match": bson.M{"business_connections.enabled": true}},
		{"$count": "n"},
	})
	if err == nil {
		defer cur.Close(ctx)
		if cur.Next(ctx) {
			var r struct {
				N int64 `bson:"n"`
			}
			if cur.Decode(&r) == nil {
				st.Connections = r.N
			}
		}
	}
	return st, nil
}

type AdminBot struct {
	ID          int64  `json:"id" bson:"_id"`
	Username    string `json:"username" bson:"username"`
	OwnerID     int64  `json:"owner_id" bson:"user_id"`
	Connections int    `json:"connections" bson:"-"`
}

func (s *Store) AdminBots(ctx context.Context) ([]AdminBot, error) {
	cur, err := s.bots.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var bots []AdminBot
	if err := cur.All(ctx, &bots); err != nil {
		return nil, err
	}
	// har bir bot uchun ulanishlar soni
	for i := range bots {
		n, _ := s.botUsers.CountDocuments(ctx, bson.M{"bot_id": bots[i].ID})
		bots[i].Connections = int(n)
	}
	if bots == nil {
		bots = []AdminBot{}
	}
	return bots, nil
}

type AdminUser struct {
	ID           int64 `json:"id"`
	Bots         int64 `json:"bots"`
	Connections  int64 `json:"connections"`
	AgentEnabled bool  `json:"agent_enabled"`
	Blocked      bool  `json:"blocked"`
	Messages     int64 `json:"messages"`
}

func (s *Store) AdminUsers(ctx context.Context) ([]AdminUser, error) {
	cur, err := s.users.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var users []struct {
		ID int64 `bson:"_id"`
	}
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}

	out := make([]AdminUser, 0, len(users))
	for _, u := range users {
		au := AdminUser{ID: u.ID}
		au.Bots, _ = s.bots.CountDocuments(ctx, bson.M{"user_id": u.ID})

		// ulanishlar
		bc, _ := s.botUsers.CountDocuments(ctx, bson.M{"user_id": u.ID})
		au.Connections = bc

		// agent yoqilganmi
		n, _ := s.configs.CountDocuments(ctx, bson.M{"_id": u.ID, "enabled": true})
		au.AgentEnabled = n > 0
		au.Blocked = s.IsBlocked(ctx, u.ID)

		out = append(out, au)
	}
	return out, nil
}

// AdminRemoveBot — superadmin istalgan botni o'chiradi (egasidan qat'i nazar).
func (s *Store) AdminRemoveBot(ctx context.Context, botID int64) error {
	if _, err := s.bots.DeleteOne(ctx, bson.M{"_id": botID}); err != nil {
		return err
	}
	_, _ = s.botUsers.DeleteMany(ctx, bson.M{"bot_id": botID})
	return nil
}

// AdminSetAgent — superadmin foydalanuvchi agentini yoqadi/o'chiradi.
func (s *Store) AdminSetAgent(ctx context.Context, ownerID int64, enabled bool) error {
	_, err := s.configs.UpdateOne(ctx, bson.M{"_id": ownerID}, bson.M{"$set": bson.M{"enabled": enabled}})
	return err
}

// --- Bloklash ---

func (s *Store) IsBlocked(ctx context.Context, uid int64) bool {
	n, _ := s.blocked.CountDocuments(ctx, bson.M{"_id": uid})
	return n > 0
}

func (s *Store) AdminSetBlocked(ctx context.Context, uid int64, blocked bool) error {
	if blocked {
		_, err := s.blocked.ReplaceOne(ctx, bson.M{"_id": uid},
			bson.M{"_id": uid, "at": time.Now()}, options.Replace().SetUpsert(true))
		// agentni o'chiramiz
		_, _ = s.configs.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{"enabled": false}})
		return err
	}
	_, err := s.blocked.DeleteOne(ctx, bson.M{"_id": uid})
	return err
}

// AdminBlockedBots — bloklanadigan foydalanuvchining botlari (to'xtatish uchun id'lar).
func (s *Store) UserBotIDs(ctx context.Context, ownerID int64) ([]int64, error) {
	cur, err := s.bots.Find(ctx, bson.M{"user_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var bots []struct {
		ID int64 `bson:"_id"`
	}
	if err := cur.All(ctx, &bots); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(bots))
	for _, b := range bots {
		ids = append(ids, b.ID)
	}
	return ids, nil
}

// AdminDeleteUser — foydalanuvchi va uning barcha ma'lumotini o'chiradi.
func (s *Store) AdminDeleteUser(ctx context.Context, ownerID int64) error {
	_, _ = s.bots.DeleteMany(ctx, bson.M{"user_id": ownerID})
	_, _ = s.botUsers.DeleteMany(ctx, bson.M{"user_id": ownerID})
	_, _ = s.configs.DeleteOne(ctx, bson.M{"_id": ownerID})
	_, _ = s.aiChats.DeleteMany(ctx, bson.M{"owner_id": ownerID})
	_, _ = s.blocked.DeleteOne(ctx, bson.M{"_id": ownerID})
	_, err := s.users.DeleteOne(ctx, bson.M{"_id": ownerID})
	return err
}

// --- Suhbatlarni ko'rish ---

func (s *Store) ownerConnIDs(ctx context.Context, ownerID int64) ([]string, error) {
	cur, err := s.botUsers.Find(ctx, bson.M{"user_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var bus []struct {
		Conns []struct {
			ID string `bson:"id"`
		} `bson:"business_connections"`
	}
	if err := cur.All(ctx, &bus); err != nil {
		return nil, err
	}
	var ids []string
	for _, bu := range bus {
		for _, c := range bu.Conns {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

type AdminChat struct {
	ChatID   int64  `json:"chat_id" bson:"_id"`
	Name     string `json:"name" bson:"name"`
	Count    int64  `json:"count" bson:"count"`
	LastText string `json:"last_text" bson:"last_text"`
}

func (s *Store) AdminUserChats(ctx context.Context, ownerID int64) ([]AdminChat, error) {
	connIDs, err := s.ownerConnIDs(ctx, ownerID)
	if err != nil || len(connIDs) == 0 {
		return []AdminChat{}, nil
	}
	pipeline := []bson.M{
		{"$match": bson.M{"message.business_connection_id": bson.M{"$in": connIDs}}},
		{"$sort": bson.M{"message.message_id": 1}},
		{"$group": bson.M{
			"_id":       "$message.chat.id",
			"count":     bson.M{"$sum": 1},
			"first":     bson.M{"$last": "$message.chat.first_name"},
			"last":      bson.M{"$last": "$message.chat.last_name"},
			"last_text": bson.M{"$last": "$message.text"},
		}},
		{"$project": bson.M{
			"count":     1,
			"last_text": 1,
			"name":      bson.M{"$trim": bson.M{"input": bson.M{"$concat": []any{bson.M{"$ifNull": []any{"$first", ""}}, " ", bson.M{"$ifNull": []any{"$last", ""}}}}}},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 100},
	}
	cur, err := s.messages.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var chats []AdminChat
	if err := cur.All(ctx, &chats); err != nil {
		return nil, err
	}
	if chats == nil {
		chats = []AdminChat{}
	}
	return chats, nil
}

type AdminMessage struct {
	FromID    int64  `json:"from_id" bson:"from_id"`
	Text      string `json:"text" bson:"text"`
	Caption   string `json:"caption" bson:"caption"`
	MessageID int    `json:"mid" bson:"mid"`
}

func (s *Store) AdminChatMessages(ctx context.Context, ownerID, chatID int64) ([]AdminMessage, error) {
	connIDs, err := s.ownerConnIDs(ctx, ownerID)
	if err != nil || len(connIDs) == 0 {
		return []AdminMessage{}, nil
	}
	pipeline := []bson.M{
		{"$match": bson.M{
			"message.chat.id":               chatID,
			"message.business_connection_id": bson.M{"$in": connIDs},
		}},
		{"$sort": bson.M{"message.message_id": 1, "message.edit_date": 1}},
		{"$group": bson.M{
			"_id": "$message.message_id",
			"doc": bson.M{"$last": "$$ROOT"},
		}},
		{"$sort": bson.M{"_id": 1}},
		{"$limit": 300},
		{"$project": bson.M{
			"_id":     0,
			"from_id": "$doc.message.from.id",
			"text":    "$doc.message.text",
			"caption": "$doc.message.caption",
			"mid":     "$doc.message.message_id",
		}},
	}
	cur, err := s.messages.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var msgs []AdminMessage
	if err := cur.All(ctx, &msgs); err != nil {
		return nil, err
	}
	if msgs == nil {
		msgs = []AdminMessage{}
	}
	return msgs, nil
}
