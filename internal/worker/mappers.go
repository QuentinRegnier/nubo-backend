package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
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
	case redis.EntityUser:
		return &UserMapper{}
	case redis.EntitySession:
		return &SessionMapper{}
	case redis.EntityUserSettings:
		return &UserSettingsMapper{}
	case redis.EntityRelation:
		return &RelationMapper{}
	case redis.EntityPost:
		return &PostMapper{}
	case redis.EntityComment:
		return &CommentMapper{}
	case redis.EntityMedia:
		return &MediaMapper{}
	case redis.EntityLike:
		return &LikeMapper{}
	case redis.EntitySaved:
		return &SavedMapper{}
	case redis.EntityMessage:
		return &MessageMapper{}
	case redis.EntityConversation:
		return &ConversationMapper{}
	case redis.EntityMembers:
		return &MemberMapper{}
	case redis.EntityReport:
		return &ReportMapper{}
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
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var u auth_models.UserPayload
	if err := json.Unmarshal(jsonBytes, &u); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs
	var phoneDB any = u.Phone
	if u.Phone == "" {
		phoneDB = nil
	}
	var birthdateDB any = u.Birthdate
	if u.Birthdate.IsZero() {
		birthdateDB = nil
	}
	var bioDB any = u.Bio
	if u.Bio == "" {
		bioDB = nil
	}
	var ppDB any = u.ProfilePictureID
	if u.ProfilePictureID == 0 {
		ppDB = nil
	}
	var locDB any = u.Location
	if u.Location == "" {
		locDB = nil
	}
	var schoolDB any = u.School
	if u.School == "" {
		schoolDB = nil
	}
	var workDB any = u.Work
	if u.Work == "" {
		workDB = nil
	}
	var banReasonDB any = u.BanReason
	if u.BanReason == "" {
		banReasonDB = nil
	}
	var banExpiresDB any = u.BanExpiresAt
	if u.BanExpiresAt.IsZero() {
		banExpiresDB = nil
	}

	return []any{
		u.ID, u.Username, u.Email, u.EmailVerified, phoneDB, u.PhoneVerified,
		u.PasswordHash, u.FirstName, u.LastName, birthdateDB, u.Sex, bioDB,
		ppDB, u.Grade, locDB, schoolDB, workDB, pq.Array(u.Badges),
		u.Desactivated, u.Banned, banReasonDB, banExpiresDB,
		u.CreatedAt, u.UpdatedAt,
	}, nil
}

func (m *UserMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- USER SETTINGS MAPPER (auth.user_settings) ---
type UserSettingsMapper struct{}

func (m *UserSettingsMapper) TableName() string { return "auth.user_settings" }

func (m *UserSettingsMapper) Columns() []string {
	return []string{
		"id", "user_id", "privacy", "notifications", "language", "theme",
		"telemetry_vector", "telemetry_tags", "telemetry_timestamp",
		"created_at", "updated_at",
	}
}

func (m *UserSettingsMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var s user_settings_models.UserSettingsPayload
	if err := json.Unmarshal(jsonBytes, &s); err != nil {
		return nil, err
	}

	privacyJSON, _ := json.Marshal(s.Privacy)
	notifJSON, _ := json.Marshal(s.Notifications)

	// TRADUCTION DES NULLs
	var telVecDB any = pq.Array(s.TelemetryVector)
	if len(s.TelemetryVector) == 0 {
		telVecDB = nil
	}
	var telTagsDB any = pq.Array(s.TelemetryTags)
	if len(s.TelemetryTags) == 0 {
		telTagsDB = nil
	}
	var telTsDB any = s.TelemetryTimestamp
	if s.TelemetryTimestamp == 0 {
		telTsDB = nil
	}

	return []any{
		s.ID, s.UserID, string(privacyJSON), string(notifJSON), s.Language, s.Theme,
		telVecDB, telTagsDB, telTsDB, s.CreatedAt, s.UpdatedAt,
	}, nil
}

func (m *UserSettingsMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- SESSION MAPPER (auth.sessions) ---
type SessionMapper struct{}

func (m *SessionMapper) TableName() string { return "auth.sessions" }

func (m *SessionMapper) Columns() []string {
	return []string{
		"id", "user_id", "master_token", "firebase_installation_id", "device_info",
		"ip_history", "current_secret", "last_secret", "last_jwt",
		"tolerance_time", "created_at", "expires_at",
	}
}

func (m *SessionMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var s auth_models.SessionsPayload
	if err := json.Unmarshal(jsonBytes, &s); err != nil {
		return nil, err
	}

	deviceInfoJSON, _ := json.Marshal(s.DeviceInfo)

	// TRADUCTION DES NULLs
	var devInfoDB any = string(deviceInfoJSON)
	if len(s.DeviceInfo) == 0 {
		devInfoDB = nil
	}
	var curSecDB any = s.CurrentSecret
	if s.CurrentSecret == "" {
		curSecDB = nil
	}
	var lastSecDB any = s.LastSecret
	if s.LastSecret == "" {
		lastSecDB = nil
	}
	var lastJwtDB any = s.LastJWT
	if s.LastJWT == "" {
		lastJwtDB = nil
	}
	var tolTimeDB any = s.ToleranceTime
	if s.ToleranceTime.IsZero() {
		tolTimeDB = nil
	}

	return []any{
		s.ID, s.UserID, s.MasterToken, s.FirebaseInstallationID, devInfoDB,
		pq.Array(s.IPHistory), curSecDB, lastSecDB, lastJwtDB,
		tolTimeDB, s.CreatedAt, s.ExpiresAt,
	}, nil
}

func (m *SessionMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- RELATION MAPPER (auth.relations) ---
type RelationMapper struct{}

func (m *RelationMapper) TableName() string { return "auth.relations" }
func (m *RelationMapper) Columns() []string {
	return []string{"id", "primary_id", "secondary_id", "state", "created_at", "updated_at"}
}
func (m *RelationMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var r relation_models.RelationPayload
	if err := json.Unmarshal(jsonBytes, &r); err != nil {
		return nil, err
	}
	return []any{r.ID, r.PrimaryID, r.SecondaryID, r.State, r.CreatedAt, r.UpdatedAt}, nil
}
func (m *RelationMapper) BuildUpdateQuery(tempTable string) string {
	return fmt.Sprintf(
		"UPDATE %s SET state = %s.state, updated_at = %s.updated_at FROM %s WHERE %s.primary_id = %s.primary_id AND %s.secondary_id = %s.secondary_id",
		m.TableName(), tempTable, tempTable, tempTable, m.TableName(), tempTable, m.TableName(), tempTable,
	)
}

// ============================================================================
//                                CONTENT SCHEMA
// ============================================================================

// --- POST MAPPER (content.posts) ---
type PostMapper struct{}

func (m *PostMapper) TableName() string { return "content.posts" }

func (m *PostMapper) Columns() []string {
	return []string{
		"id", "user_id", "content", "hashtags", "identifiers", "media_ids",
		"visibility", "priority_level", "location", "like_count",
		"comment_count", "view_count", "report_count", "has_media",
		"vector", "vector_version", "telemetry_dwell_sum", "telemetry_dwell_sq",
		"telemetry_clicks", "created_at", "updated_at",
	}
}

func (m *PostMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var p post_models.PostPayload
	if err := json.Unmarshal(jsonBytes, &p); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs
	var contentDB any = p.Content
	if p.Content == "" {
		contentDB = nil
	}
	var locationDB any = p.Location
	if p.Location == "" {
		locationDB = nil
	}
	var vectorDB any = pq.Array(p.Vector)
	if len(p.Vector) == 0 {
		vectorDB = nil
	}

	return []any{
		p.ID, p.UserID, contentDB, pq.Array(p.Hashtags), pq.Array(p.Identifiers), pq.Array(p.MediaIDs),
		p.Visibility, p.PriorityLevel, locationDB, p.LikeCount, p.CommentCount,
		p.ViewCount, p.ReportCount, p.HasMedia, vectorDB, p.VectorVersion, p.TelemetryDwellSum,
		p.TelemetryDwellSq, p.TelemetryClicks, p.CreatedAt, p.UpdatedAt,
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
	var med media_models.MediaPayload
	if err := json.Unmarshal(jsonBytes, &med); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs
	var storageDB any = med.StoragePath
	if med.StoragePath == "" {
		storageDB = nil
	}

	return []any{
		med.ID, med.OwnerID, storageDB, med.Visibility, med.CreatedAt, med.UpdatedAt,
	}, nil
}

func (m *MediaMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- COMMENT MAPPER (content.comments) ---
type CommentMapper struct{}

func (m *CommentMapper) TableName() string { return "content.comments" }

func (m *CommentMapper) Columns() []string {
	return []string{"id", "post_id", "user_id", "content", "visibility", "like_count", "score", "created_at", "updated_at"}
}

func (m *CommentMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var c comment_models.CommentPayload
	if err := json.Unmarshal(jsonBytes, &c); err != nil {
		return nil, err
	}
	return []any{c.ID, c.PostID, c.UserID, c.Content, c.Visibility, c.LikeCount, c.Score, c.CreatedAt, c.UpdatedAt}, nil
}

func (m *CommentMapper) BuildUpdateQuery(t string) string {
	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
}

// --- LIKE MAPPER (content.likes) ---
type LikeMapper struct{}

func (m *LikeMapper) TableName() string { return "content.likes" }

func (m *LikeMapper) Columns() []string {
	return []string{"id", "target_type", "target_id", "user_id", "created_at"}
}

type LikeWorkerPayload struct {
	ID         int64  `json:"id"`
	TargetType int    `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	UserID     int64  `json:"user_id"`
	CreatedAt  string `json:"created_at"`
}

func (m *LikeMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var l LikeWorkerPayload
	if err := json.Unmarshal(jsonBytes, &l); err != nil {
		return nil, err
	}

	return []any{l.ID, l.TargetType, l.TargetID, l.UserID, l.CreatedAt}, nil
}

func (m *LikeMapper) BuildUpdateQuery(_ string) string { return "" }

// --- SAVED MAPPER (content.saved) ---
type SavedMapper struct{}

func (m *SavedMapper) TableName() string { return "content.saved" }

func (m *SavedMapper) Columns() []string {
	return []string{"id", "user_id", "post_id", "created_at"}
}

func (m *SavedMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var s saved_models.SavedPayload
	if err := json.Unmarshal(jsonBytes, &s); err != nil {
		return nil, err
	}
	return []any{s.ID, s.UserID, s.PostID, s.CreatedAt}, nil
}

func (m *SavedMapper) BuildUpdateQuery(_ string) string { return "" }

// ============================================================================
//                                MESSAGING SCHEMA
// ============================================================================

// --- MESSAGE MAPPER (messaging.messages) ---
type MessageMapper struct{}

func (m *MessageMapper) TableName() string { return "messaging.messages" }

func (m *MessageMapper) Columns() []string {
	return []string{"id", "conversation_id", "sender_id", "message_type", "visibility", "content", "attachments", "created_at", "updated_at"}
}

func (m *MessageMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var msg message_models.MessagePayload
	if err := json.Unmarshal(jsonBytes, &msg); err != nil {
		return nil, err
	}

	attachJSON, _ := json.Marshal(msg.Attachments)

	// TRADUCTION DES NULLs
	var contentDB any = msg.Content
	if msg.Content == "" {
		contentDB = nil
	}
	var attachDB any = string(attachJSON)
	if len(msg.Attachments) == 0 {
		attachDB = nil
	}

	return []any{
		msg.ID, msg.ConversationID, msg.SenderID, msg.MessageType, msg.Visibility, contentDB, attachDB, msg.CreatedAt, msg.UpdatedAt,
	}, nil
}

func (m *MessageMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

// --- CONVERSATION MAPPER (messaging.conversations) ---
type ConversationMapper struct{}

func (m *ConversationMapper) TableName() string { return "messaging.conversations" }

func (m *ConversationMapper) Columns() []string {
	return []string{"id", "type", "title", "last_message_id", "state", "laws", "created_at", "updated_at"}
}

func (m *ConversationMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var c conversation_models.ConversationPayload
	if err := json.Unmarshal(jsonBytes, &c); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs
	var titleDB any = c.Title
	if c.Title == "" {
		titleDB = nil
	}
	var lastMsgDB any = c.LastMessageID
	if c.LastMessageID == 0 {
		lastMsgDB = nil
	}

	return []any{c.ID, c.Type, titleDB, lastMsgDB, c.State, pq.Array(c.Laws), c.CreatedAt, c.UpdatedAt}, nil
}

func (m *ConversationMapper) BuildUpdateQuery(t string) string {
	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
}

// --- MEMBER MAPPER (messaging.members) ---
type MemberMapper struct{}

func (m *MemberMapper) TableName() string {
	return "messaging.members"
}

func (m *MemberMapper) Columns() []string {
	return []string{"id", "conversation_id", "user_id", "role", "joined_at", "unread_count", "frozen_message_id", "created_at", "updated_at"}
}

func (m *MemberMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var mem conversation_models.MemberPayload
	if err := json.Unmarshal(jsonBytes, &mem); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs (Si 0, on met NULL en base)
	var frozenDB any = mem.FrozenMessageID
	if mem.FrozenMessageID == 0 {
		frozenDB = nil
	}

	return []any{mem.ID, mem.ConversationID, mem.UserID, mem.Role, mem.JoinedAt, mem.UnreadCount, frozenDB, mem.CreatedAt, mem.UpdatedAt}, nil
}

func (m *MemberMapper) BuildUpdateQuery(t string) string {
	return buildGenericUpdateQuery(m.TableName(), t, m.Columns())
}

// ============================================================================
//                                MODERATION SCHEMA
// ============================================================================

// --- REPORT MAPPER (moderation.reports) ---
type ReportMapper struct{}

func (m *ReportMapper) TableName() string { return "moderation.reports" }

func (m *ReportMapper) Columns() []string {
	return []string{
		"id", "reporter_id", "target_type", "target_ids", "category",
		"reason", "rationale", "state", "created_at", "updated_at",
	}
}

func (m *ReportMapper) ToRow(data any) ([]any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var r report_models.ReportPayload
	if err := json.Unmarshal(jsonBytes, &r); err != nil {
		return nil, err
	}

	// TRADUCTION DES NULLs
	var reasonDB any = r.Reason
	if r.Reason == "" {
		reasonDB = nil
	}
	var rationaleDB any = r.Rationale
	if r.Rationale == "" {
		rationaleDB = nil
	}

	return []any{
		r.ID, r.ReporterID, r.TargetType, pq.Array(r.TargetIDs),
		r.Category, reasonDB, rationaleDB, r.State, r.CreatedAt, r.UpdatedAt,
	}, nil
}

func (m *ReportMapper) BuildUpdateQuery(tempTable string) string {
	return buildGenericUpdateQuery(m.TableName(), tempTable, m.Columns())
}

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
		tableName, strings.Join(sets, ", "), tempTable, tableName, tempTable,
	)
}
