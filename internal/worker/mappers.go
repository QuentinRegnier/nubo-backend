package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/lib/pq"
)

// EntityMapper définit comment transformer un struct en ligne SQL.
type EntityMapper interface {
	TableName() string
	Columns() []string
	ToRow(data any) ([]any, error)
	BuildUpdateQuery(tempTable string) string
}

// GetMapper retourne le mapper correspondant au type d'entité Redis.
func GetMapper(entity redis.EntityType) EntityMapper {
	switch entity {
	// --- AUTH ---
	case redis.EntityUser:
		return &UserMapper{}
	case redis.EntitySession:
		return &SessionMapper{}
	// case redis.EntityRelation:
	// 	return &RelationMapper{}

	// --- CONTENT ---
	case redis.EntityPost:
		return &PostMapper{}
	// case redis.EntityComment:
	// 	return &CommentMapper{}
	case redis.EntityMedia:
		return &MediaMapper{}
	// case redis.EntityLike: // Assure-toi d'avoir cette constante dans redis/async_queue.go
	// 	return &LikeMapper{}

	// // --- MESSAGING ---
	// case redis.EntityMessage:
	// 	return &MessageMapper{}
	// case redis.EntityConversation:
	// 	return &ConversationMapper{}
	// case redis.EntityMembers:
	// 	return &MemberMapper{}

	default:
		return nil
	}
}

// ============================================================================
//                                 AUTH SCHEMA
// ============================================================================

// --- USER MAPPER (auth.users) ---
type UserMapper struct{}

func (m *UserMapper) TableName() string { return "auth.users" }

func (m *UserMapper) Columns() []string {
	return []string{
		"id", "username", "email", "email_verified", "phone", "phone_verified",
		"password_hash", "first_name", "last_name", "birthdate", "sex", "bio",
		"profile_picture_id", "grade", "location", "school", "work", "badges",
		"desactivated", "banned", "ban_reason", "ban_expires_at",
		"created_at", "updated_at",
	}
}

func (m *UserMapper) ToRow(data any) ([]any, error) {
	// Hack JSON pour convertir map[string]interface{} (Redis) -> Struct
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var u domain.UserRequest
	if err := json.Unmarshal(jsonBytes, &u); err != nil {
		return nil, err
	}

	// Conversion Date pour SQL (si vide)
	// Attention aux champs optionnels (Pointers ou Zero values)
	return []any{
		u.ID, u.Username, u.Email, u.EmailVerified, u.Phone, u.PhoneVerified,
		u.PasswordHash, u.FirstName, u.LastName, u.Birthdate, u.Sex, u.Bio,
		u.ProfilePictureID, u.Grade, u.Location, u.School, u.Work, pq.Array(u.Badges),
		u.Desactivated, u.Banned, u.BanReason, u.BanExpiresAt,
		u.CreatedAt, u.UpdatedAt,
	}, nil
}

func (m *UserMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- SESSION MAPPER (auth.sessions) ---
type SessionMapper struct{}

func (m *SessionMapper) TableName() string { return "auth.sessions" }

func (m *SessionMapper) Columns() []string {
	return []string{
		"id", "user_id", "master_token", "device_token", "device_info",
		"ip_history", "current_secret", "last_secret", "last_jwt",
		"tolerance_time", "created_at", "expires_at",
	}
}

func (m *SessionMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var s domain.SessionsRequest
	if err := json.Unmarshal(jsonBytes, &s); err != nil {
		return nil, err
	}

	deviceInfoJSON, err := json.Marshal(s.DeviceInfo)
	if err != nil {
		return nil, err
	}

	return []any{
		s.ID, s.UserID, s.MasterToken, s.DeviceToken, string(deviceInfoJSON),
		pq.Array(s.IPHistory), s.CurrentSecret, s.LastSecret, s.LastJWT,
		s.ToleranceTime, s.CreatedAt, s.ExpiresAt,
	}, nil
}

func (m *SessionMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// // --- RELATION MAPPER (auth.relations) ---
// type RelationMapper struct{}

// func (m *RelationMapper) TableName() string { return "auth.relations" }

// func (m *RelationMapper) Columns() []string {
// 	return []string{"id", "follower_id", "followed_id", "state", "created_at", "updated_at"}
// }

// func (m *RelationMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var r domain.Relation // Assure-toi d'avoir ce struct
// 	json.Unmarshal(jsonBytes, &r)

// 	return []any{
// 		r.ID, r.FollowerID, r.FollowedID, r.State, r.CreatedAt, r.UpdatedAt,
// 	}
// }

// func (m *RelationMapper) BuildUpdateQuery(tempTable string) string {
// 	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
// }

// ============================================================================
//                                CONTENT SCHEMA
// ============================================================================

// --- POST MAPPER (content.posts) ---
type PostMapper struct{}

func (m *PostMapper) TableName() string { return "content.posts" }

func (m *PostMapper) Columns() []string {
	// Correspondance exacte avec votre schéma de table
	return []string{"id", "user_id", "content", "hashtags", "identifiers", "media_ids", "visibility", "location", "created_at", "updated_at"}
}

func (m *PostMapper) ToRow(data any) ([]any, error) {
	// Conversion sécurisée des données venant de Redis
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var p struct {
		ID          int64     `json:"id"`
		UserID      int64     `json:"user_id"`
		Content     string    `json:"content"`
		Hashtags    []string  `json:"hashtags"`
		Identifiers []int64   `json:"identifiers"`
		MediaIDs    []int64   `json:"media_ids"`
		Visibility  int       `json:"visibility"`
		Location    string    `json:"location"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(jsonBytes, &p); err != nil {
		return nil, err
	}

	return []any{
		p.ID,
		p.UserID,
		p.Content,
		pq.Array(p.Hashtags), // Utilisation de pq.Array pour les types array SQL
		pq.Array(p.Identifiers),
		pq.Array(p.MediaIDs),
		p.Visibility,
		p.Location,
		p.CreatedAt,
		p.UpdatedAt,
	}, nil
}

func (m *PostMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- MEDIA MAPPER (content.media) ---
type MediaMapper struct{}

func (m *MediaMapper) TableName() string { return "content.media" }

func (m *MediaMapper) Columns() []string {
	return []string{"id", "owner_id", "storage_path", "visibility", "created_at", "updated_at"}
}

func (m *MediaMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var med domain.MediaRequest
	if err := json.Unmarshal(jsonBytes, &med); err != nil {
		return nil, err
	}

	return []any{
		med.ID, med.OwnerID, med.StoragePath, med.Visibility, med.CreatedAt, med.UpdatedAt,
	}, nil
}

func (m *MediaMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// // --- COMMENT MAPPER (content.comments) ---
// type CommentMapper struct{}

// func (m *CommentMapper) TableName() string { return "content.comments" }
// func (m *CommentMapper) Columns() []string {
// 	return []string{"id", "post_id", "user_id", "content", "visibility", "created_at", "updated_at"}
// }
// func (m *CommentMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var c domain.Comment
// 	json.Unmarshal(jsonBytes, &c)
// 	return []any{c.ID, c.PostID, c.UserID, c.Content, c.Visibility, c.CreatedAt, c.UpdatedAt}
// }
// func (m *CommentMapper) BuildUpdateQuery(t string) string {
// 	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
// }

// // --- LIKE MAPPER (content.likes) ---
// type LikeMapper struct{}

// func (m *LikeMapper) TableName() string { return "content.likes" }
// func (m *LikeMapper) Columns() []string {
// 	return []string{"id", "target_type", "target_id", "user_id", "created_at"}
// }
// func (m *LikeMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var l domain.Like
// 	json.Unmarshal(jsonBytes, &l)
// 	return []any{l.ID, l.TargetType, l.TargetID, l.UserID, l.CreatedAt}
// }

// // Pas d'update sur les likes, mais on met la fonction pour l'interface
// func (m *LikeMapper) BuildUpdateQuery(t string) string { return "" }

// // ============================================================================
// //                                MESSAGING SCHEMA
// // ============================================================================

// // --- MESSAGE MAPPER (messaging.messages) ---
// type MessageMapper struct{}

// func (m *MessageMapper) TableName() string { return "messaging.messages" }

// func (m *MessageMapper) Columns() []string {
// 	return []string{"id", "conversation_id", "sender_id", "message_type", "state", "content", "attachments", "created_at", "updated_at"}
// }

// func (m *MessageMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var msg domain.Message
// 	json.Unmarshal(jsonBytes, &msg)

// 	attachJSON, _ := json.Marshal(msg.Attachments)

// 	return []any{
// 		msg.ID, msg.ConversationID, msg.SenderID, msg.MessageType, msg.State,
// 		msg.Content, string(attachJSON), msg.CreatedAt, msg.UpdatedAt,
// 	}
// }

// func (m *MessageMapper) BuildUpdateQuery(tempTable string) string {
// 	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
// }

// // --- CONVERSATION MAPPER (messaging.conversations) ---
// type ConversationMapper struct{}

// func (m *ConversationMapper) TableName() string { return "messaging.conversations" }
// func (m *ConversationMapper) Columns() []string {
// 	return []string{"id", "type", "title", "last_message_id", "last_read_by_all_message_id", "state", "created_at", "updated_at"}
// }
// func (m *ConversationMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var c domain.Conversation
// 	json.Unmarshal(jsonBytes, &c)
// 	return []any{c.ID, c.Type, c.Title, c.LastMessageID, c.LastReadByAllMessageID, c.State, c.CreatedAt, c.UpdatedAt}
// }
// func (m *ConversationMapper) BuildUpdateQuery(t string) string {
// 	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
// }

// // --- MEMBER MAPPER (messaging.members) ---
// type MemberMapper struct{}

// func (m *MemberMapper) TableName() string { return "messaging.members" }
// func (m *MemberMapper) Columns() []string {
// 	return []string{"id", "conversation_id", "user_id", "role", "joined_at", "unread_count", "created_at", "updated_at"}
// }
// func (m *MemberMapper) ToRow(data any) []any {
// 	jsonBytes, _ := json.Marshal(data)
// 	var mem domain.Member
// 	json.Unmarshal(jsonBytes, &mem)
// 	return []any{mem.ID, mem.ConversationID, mem.UserID, mem.Role, mem.JoinedAt, mem.UnreadCount, mem.CreatedAt, mem.UpdatedAt}
// }
// func (m *MemberMapper) BuildUpdateQuery(t string) string {
// 	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
// }

// ============================================================================
//                                UTILITAIRES
// ============================================================================

// buildGenericUpdateQuery génère la requête SQL "UPDATE ... FROM temp_table" automatiquement
func buildGenericUpdateQuery(tableName, tempTable string, columns []string) string {
	var sets []string
	for _, c := range columns {
		if c == "id" {
			continue // On ne met jamais à jour la Primary Key
		}
		sets = append(sets, fmt.Sprintf("%s = %s.%s", c, tempTable, c))
	}

	return fmt.Sprintf(
		"UPDATE %s SET %s FROM %s WHERE %s.id = %s.id",
		tableName,
		strings.Join(sets, ", "),
		tempTable,
		tableName,
		tempTable,
	)
}
