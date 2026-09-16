package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/lib/pq"
)

// GenerateCopyQuery crée une requête COPY qui respecte les schémas (ex: auth.users)
func GenerateCopyQuery(fullTableName string, columns []string) string {
	quotedTableName := ""
	if strings.Contains(fullTableName, ".") {
		parts := strings.SplitN(fullTableName, ".", 2)
		quotedTableName = pq.QuoteIdentifier(parts[0]) + "." + pq.QuoteIdentifier(parts[1])
	} else {
		quotedTableName = pq.QuoteIdentifier(fullTableName)
	}

	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = pq.QuoteIdentifier(col)
	}

	return fmt.Sprintf("COPY %s (%s) FROM STDIN", quotedTableName, strings.Join(quotedColumns, ", "))
}

func flushPostgres(ctx context.Context, events []redis.AsyncEvent) {
	grouped := make(map[redis.EntityType][]redis.AsyncEvent)
	for _, e := range events {
		grouped[e.Type] = append(grouped[e.Type], e)
	}

	// L'ordre d'exécution topologique garantit que les parents sont insérés avant les enfants.
	executionOrder := []redis.EntityType{
		redis.EntityUser,
		redis.EntityUserSettings,
		redis.EntitySession,
		redis.EntityRelation,
		redis.EntityPost,
		redis.EntityComment,
		redis.EntityLike,
		redis.EntityMedia,
		redis.EntityConversation,
		redis.EntityMembers,
		redis.EntityMessage,
	}

	// 1. Exécution ordonnée
	for _, entityType := range executionOrder {
		if events, exists := grouped[entityType]; exists {
			processEntityEvents(ctx, entityType, events)
			delete(grouped, entityType)
		}
	}

	// 2. Exécution du reste
	for entityType, evts := range grouped {
		processEntityEvents(ctx, entityType, evts)
	}
	updateCountersPostgres(ctx, events)
}

// processEntityEvents gère le Fast Path et déclenche le Slow Path en cas d'erreur.
func processEntityEvents(ctx context.Context, entityType redis.EntityType, events []redis.AsyncEvent) {
	var inserts, updates, deletes []redis.AsyncEvent

	for _, e := range events {
		switch e.Action {
		case redis.ActionCreate:
			inserts = append(inserts, e)
		case redis.ActionUpdate:
			updates = append(updates, e)
		case redis.ActionDelete:
			deletes = append(deletes, e)
		}
	}

	if len(inserts) > 0 {
		if err := bulkInsertPostgres(ctx, entityType, inserts); err != nil {
			logger.Log.Warn().Err(err).Interface("entity", entityType).Msg("Fast Path Insert failed. Déclenchement Dichotomie...")
			slowPathDichotomy(ctx, entityType, redis.ActionCreate, inserts)
		}
	}
	if len(updates) > 0 {
		if err := bulkUpdatePostgres(ctx, entityType, updates); err != nil {
			logger.Log.Warn().Err(err).Interface("entity", entityType).Msg("Fast Path Update failed. Déclenchement Dichotomie...")
			slowPathDichotomy(ctx, entityType, redis.ActionUpdate, updates)
		}
	}
	if len(deletes) > 0 {
		if err := bulkDeletePostgres(ctx, entityType, deletes); err != nil {
			logger.Log.Warn().Err(err).Interface("entity", entityType).Msg("Fast Path Delete failed. Déclenchement Dichotomie...")
			slowPathDichotomy(ctx, entityType, redis.ActionDelete, deletes)
		}
	}
}

// ============================================================================
// RÉSILIENCE : DICHOTOMIE & DLQ
// ============================================================================

// slowPathDichotomy divise récursivement un batch en échec pour isoler la requête corrompue.
func slowPathDichotomy(ctx context.Context, entity redis.EntityType, action redis.ActionType, events []redis.AsyncEvent) {
	if len(events) == 0 {
		return
	}

	// 1. Tenter d'exécuter ce sous-lot
	var err error
	switch action {
	case redis.ActionCreate:
		err = bulkInsertPostgres(ctx, entity, events)
	case redis.ActionUpdate:
		err = bulkUpdatePostgres(ctx, entity, events)
	case redis.ActionDelete:
		err = bulkDeletePostgres(ctx, entity, events)
	}

	// 2. Si ça passe, on a sauvé ce sous-lot, on arrête ici.
	if err == nil {
		return
	}

	// 3. Si on échoue et qu'il n'y a qu'UN SEUL élément, c'est le coupable !
	if len(events) == 1 {
		sendToDLQ(ctx, entity, action, events[0], err)
		return
	}

	// 4. Si on échoue avec plusieurs éléments, on coupe en deux et on relance.
	mid := len(events) / 2
	slowPathDichotomy(ctx, entity, action, events[:mid])
	slowPathDichotomy(ctx, entity, action, events[mid:])
}

// sendToDLQ envoie la requête empoisonnée dans une file de quarantaine sur Redis.
func sendToDLQ(ctx context.Context, entity redis.EntityType, action redis.ActionType, event redis.AsyncEvent, dbErr error) {
	dlqPayload := map[string]any{
		"nubo_error": dbErr.Error(),
		"time":       time.Now().Format(time.RFC3339),
		"entity":     entity,
		"action":     action,
		"event":      event,
	}

	bytes, err := json.Marshal(dlqPayload)
	if err == nil {
		_ = redis.DLQ.LPush(ctx, "postgres_errors", bytes)
		logger.Log.Error().Err(dbErr).
			Interface("entity", entity).
			Interface("action", action).
			Int64("event_id", event.ID).
			Msg("Événement empoisonné isolé et mis en quarantaine (DLQ)")
	}
}

// ============================================================================
// 1. BULK INSERT (Via lib/pq CopyIn)
// ============================================================================
func bulkInsertPostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	// CAS SPÉCIAL : LES RÉACTIONS AUX MESSAGES (UPSERT)
	if entity == redis.EntityMessageReaction {
		return handleMessageReactionUpsert(ctx, events)
	}

	mapper := GetMapper(entity)
	if mapper == nil {
		return nubo_error.NewInternal(errors.New("pas de mapper Postgres"))
	}

	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	copyQuery := GenerateCopyQuery(mapper.TableName(), mapper.Columns())

	stmt, err := tx.Prepare(copyQuery)
	if err != nil {
		return nubo_error.NewInternal(err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture statement CopyIn")
		}
	}(stmt)

	for _, e := range events {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			return nubo_error.NewInternal(err)
		}
		if _, err = stmt.Exec(row...); err != nil {
			return nubo_error.NewInternal(err)
		}
	}

	// Flush du COPY
	if _, err := stmt.Exec(); err != nil {
		return nubo_error.NewInternal(err)
	}

	if err := tx.Commit(); err != nil {
		return nubo_error.NewInternal(err)
	}

	committed = true
	return nil
}

func handleMessageReactionUpsert(ctx context.Context, events []redis.AsyncEvent) error {
	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO messaging.message_reactions (id, message_id, user_id, reaction, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id, user_id) 
		DO UPDATE SET reaction = EXCLUDED.reaction, created_at = EXCLUDED.created_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return nubo_error.NewInternal(err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture statement MessageReaction Upsert")
		}
	}(stmt)

	for _, e := range events {
		jsonBytes, _ := json.Marshal(e.Payload)
		var r message_models.MessageReactionPayload
		_ = json.Unmarshal(jsonBytes, &r)

		_, err = stmt.Exec(r.ID, r.MessageID, r.UserID, r.Reaction, r.CreatedAt)
		if err != nil {
			_ = tx.Rollback()
			return nubo_error.NewInternal(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return nubo_error.NewInternal(err)
	}
	return nil
}

// ============================================================================
// 2. BULK UPDATE (Temp Table + COPY)
// ============================================================================
func bulkUpdatePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	mapper := GetMapper(entity)
	if mapper == nil {
		return nubo_error.NewInternal(errors.New("pas de mapper Postgres"))
	}

	// --- DÉDUPLICATION RAM (Last-Write-Wins) ---
	// Résout le comportement indéterministe de Postgres lors d'un UPDATE ... FROM
	// avec des clés primaires dupliquées dans la table temporaire.
	dedupMap := make(map[int64]redis.AsyncEvent)
	for _, e := range events {
		dedupMap[e.ID] = e
	}

	dedupEvents := make([]redis.AsyncEvent, 0, len(dedupMap))
	for _, e := range dedupMap {
		dedupEvents = append(dedupEvents, e)
	}

	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	safeTableName := strings.ReplaceAll(mapper.TableName(), ".", "_")
	tempTable := fmt.Sprintf("tmp_%s_%d", safeTableName, time.Now().UnixNano())

	queryCreateTable := fmt.Sprintf("CREATE TEMP TABLE %s (LIKE %s INCLUDING ALL) ON COMMIT DROP", tempTable, mapper.TableName())
	if _, err := tx.ExecContext(ctx, queryCreateTable); err != nil {
		return nubo_error.NewInternal(err)
	}

	stmt, err := tx.Prepare(pq.CopyIn(tempTable, mapper.Columns()...))
	if err != nil {
		return nubo_error.NewInternal(err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture statement CopyIn Temp")
		}
	}(stmt)

	// On itère strictement sur les événements dédupliqués
	for _, e := range dedupEvents {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			return nubo_error.NewInternal(err)
		}
		if _, err := stmt.Exec(row...); err != nil {
			return nubo_error.NewInternal(err)
		}
	}

	if _, err := stmt.Exec(); err != nil {
		return nubo_error.NewInternal(err)
	}

	queryUpdate := mapper.BuildUpdateQuery(tempTable)
	if _, err := tx.ExecContext(ctx, queryUpdate); err != nil {
		return nubo_error.NewInternal(err)
	}

	if err := tx.Commit(); err != nil {
		return nubo_error.NewInternal(err)
	}

	committed = true
	return nil
}

// ============================================================================
// 3. BULK DELETE
// ============================================================================
func bulkDeletePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	mapper := GetMapper(entity)
	if mapper == nil {
		return nubo_error.NewInternal(errors.New("pas de mapper Postgres"))
	}

	// 🚨 CAS SPÉCIAL : LES LIKES (Suppression par clé composite)
	if entity == redis.EntityLike {
		tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
		if err != nil {
			return nubo_error.NewInternal(err)
		}

		stmt, err := tx.Prepare("DELETE FROM content.likes WHERE target_type = $1 AND target_id = $2 AND user_id = $3")
		if err != nil {
			_ = tx.Rollback()
			return nubo_error.NewInternal(err)
		}
		defer func(stmt *sql.Stmt) {
			err := stmt.Close()
			if err != nil {
				logger.Log.Error().Err(err).Msg("Erreur fermeture statement Delete Likes")
			}
		}(stmt)

		for _, e := range events {
			jsonBytes, _ := json.Marshal(e.Payload)
			var l map[string]interface{}
			_ = json.Unmarshal(jsonBytes, &l)

			// Extraction robuste des valeurs flottantes du JSON
			targetType := int(l["target_type"].(float64))
			targetID := int64(l["target_id"].(float64))
			userID := int64(l["user_id"].(float64))

			_, _ = stmt.Exec(targetType, targetID, userID)
		}

		if err := tx.Commit(); err != nil {
			return nubo_error.NewInternal(err)
		}
		return nil
	}

	// CAS SPÉCIAL : LES RELATIONS (Suppression par clé composite)
	if entity == redis.EntityRelation {
		tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
		if err != nil {
			return nubo_error.NewInternal(err)
		}
		stmt, err := tx.Prepare("DELETE FROM auth.relations WHERE primary_id = $1 AND secondary_id = $2")
		if err != nil {
			_ = tx.Rollback()
			return nubo_error.NewInternal(err)
		}
		defer func(stmt *sql.Stmt) {
			err := stmt.Close()
			if err != nil {
				logger.Log.Error().Err(err).Msg("Erreur fermeture statement Delete Relations")
			}
		}(stmt)

		for _, e := range events {
			jsonBytes, _ := json.Marshal(e.Payload)
			var r map[string]interface{}
			_ = json.Unmarshal(jsonBytes, &r)

			// Extraction robuste
			primaryID := int64(r["primary_id"].(float64))
			secondaryID := int64(r["secondary_id"].(float64))
			_, _ = stmt.Exec(primaryID, secondaryID)
		}
		if err := tx.Commit(); err != nil {
			return nubo_error.NewInternal(err)
		}
		return nil
	}

	// CAS SPÉCIAL : LES RÉACTIONS AUX MESSAGES (Suppression par clé composite)
	if entity == redis.EntityMessageReaction {
		tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
		if err != nil {
			return nubo_error.NewInternal(err)
		}

		stmt, err := tx.Prepare("DELETE FROM messaging.message_reactions WHERE message_id = $1 AND user_id = $2")
		if err != nil {
			_ = tx.Rollback()
			return nubo_error.NewInternal(err)
		}
		defer func(stmt *sql.Stmt) {
			err := stmt.Close()
			if err != nil {
				logger.Log.Error().Err(err).Msg("Erreur fermeture statement Delete Message Reactions")
			}
		}(stmt)

		for _, e := range events {
			jsonBytes, _ := json.Marshal(e.Payload)
			var r message_models.MessageReactionPayload
			_ = json.Unmarshal(jsonBytes, &r)

			_, err = stmt.Exec(r.MessageID, r.UserID)
			if err != nil {
				_ = tx.Rollback()
				return nubo_error.NewInternal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			return nubo_error.NewInternal(err)
		}
		return nil
	}

	// CAS SPÉCIAL : LES FAVORIS (Suppression par clé composite)
	if entity == redis.EntitySaved {
		tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
		if err != nil {
			return nubo_error.NewInternal(err)
		}
		stmt, err := tx.Prepare("DELETE FROM content.saved WHERE user_id = $1 AND post_id = $2")
		if err != nil {
			_ = tx.Rollback()
			return nubo_error.NewInternal(err)
		}
		defer func(stmt *sql.Stmt) {
			err := stmt.Close()
			if err != nil {
				logger.Log.Error().Err(err).Msg("Erreur fermeture statement Delete Saved")
			}
		}(stmt)

		for _, e := range events {
			jsonBytes, _ := json.Marshal(e.Payload)
			var s map[string]interface{}
			_ = json.Unmarshal(jsonBytes, &s)

			userID := int64(s["user_id"].(float64))
			postID := int64(s["post_id"].(float64))
			_, _ = stmt.Exec(userID, postID)
		}
		if err := tx.Commit(); err != nil {
			return nubo_error.NewInternal(err)
		}
		return nil
	}

	// COMPORTEMENT STANDARD (Par tableau d'IDs)
	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	var query string

	if entity == redis.EntityPost {
		// 1. SOFT DELETE des posts
		query = fmt.Sprintf("UPDATE %s SET visibility = -1 WHERE id = ANY($1)", mapper.TableName())
		_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(ids))
		if err != nil {
			return nubo_error.NewInternal(err)
		}

		// 2. CASCADE SQL : Soft Delete des commentaires associés
		_, _ = postgres.PostgresDB.ExecContext(ctx, "UPDATE content.comments SET visibility = -1, updated_at = NOW() WHERE post_id = ANY($1)", pq.Array(ids))

		// 3. CASCADE SQL : Hard Delete des likes associés
		_, _ = postgres.PostgresDB.ExecContext(ctx, "DELETE FROM content.likes WHERE target_type = 0 AND target_id = ANY($1)", pq.Array(ids))

		// ✅ NOUVEAU - 3.5. CASCADE SQL : Hard Delete des favoris associés
		_, _ = postgres.PostgresDB.ExecContext(ctx, "DELETE FROM content.saved WHERE post_id = ANY($1)", pq.Array(ids))

		// 4. CASCADE SQL : Soft Delete des Médias associés (visibility = false)
		_, _ = postgres.PostgresDB.ExecContext(ctx, "UPDATE content.media SET visibility = false, updated_at = NOW() WHERE id IN (SELECT unnest(media_ids) FROM content.posts WHERE id = ANY($1))", pq.Array(ids))

		return nil
	} else if entity == redis.EntityMessage { // ✅ NOUVEAU BLOC
		// SOFT DELETE du Message
		query = fmt.Sprintf("UPDATE %s SET visibility = false, updated_at = NOW() WHERE id = ANY($1)", mapper.TableName())
		_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(ids))
		if err != nil {
			return nubo_error.NewInternal(err)
		}

		// HARD DELETE des réactions associées pour économiser la BDD
		_, _ = postgres.PostgresDB.ExecContext(ctx, "DELETE FROM messaging.message_reactions WHERE message_id = ANY($1)", pq.Array(ids))

		return nil
	} else if entity == redis.EntityComment {
		// SOFT DELETE pour les commentaires effacés unitairement
		query = fmt.Sprintf("UPDATE %s SET visibility = -1 WHERE id = ANY($1)", mapper.TableName())

	} else {
		// HARD DELETE pour le reste des entités standards
		query = fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", mapper.TableName())
	}

	// UNIFICATION DE L'EXÉCUTION
	_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nubo_error.NewInternal(err)
	}
	return nil
}

// updateCountersPostgres regroupe les événements et met à jour les colonnes 'en dur' de la table posts.
func updateCountersPostgres(ctx context.Context, events []redis.AsyncEvent) {
	likeDeltas := make(map[int64]int)
	commentDeltas := make(map[int64]int)
	viewDeltas := make(map[int64]int)
	commentLikeDeltas := make(map[int64]int)
	reportDeltas := make(map[int64]int)
	telemetryDwellSum := make(map[int64]float64)
	telemetryDwellSq := make(map[int64]float64)
	telemetryClicks := make(map[int64]int)

	// Remplacement de l'ancien système par le nouveau buffer
	messageReactionDeltas := make(map[int64]map[string]int)

	for _, e := range events {
		delta := 1
		if e.Action == redis.ActionDelete {
			delta = -1
		}

		jsonBytes, err := json.Marshal(e.Payload)
		if err != nil {
			logger.Log.Error().Err(err).Interface("payload", e.Payload).Msg("Erreur Marshal Payload dans updateCountersPostgres")
			continue
		}

		if e.Type == redis.EntityLike {
			var p struct {
				TargetType int   `json:"target_type"`
				TargetID   int64 `json:"target_id"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil {
				if p.TargetType == 0 && p.TargetID != 0 {
					likeDeltas[p.TargetID] += delta
				} else if p.TargetType == 1 && p.TargetID != 0 {
					commentLikeDeltas[p.TargetID] += delta
				}
			}
		} else if e.Type == redis.EntityComment {
			var p struct {
				PostID int64 `json:"post_id"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.PostID != 0 {
				commentDeltas[p.PostID] += delta
			}
		} else if e.Type == redis.EntityView {
			var p struct {
				TargetID int64 `json:"target_id"`
				Count    int   `json:"count"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.TargetID != 0 {
				if p.Count != 0 {
					delta = p.Count
				}
				viewDeltas[p.TargetID] += delta
			}
		} else if e.Type == redis.EntityReport {
			var r struct {
				TargetType int     `json:"target_type"`
				TargetIDs  []int64 `json:"target_ids"`
			}
			if err := json.Unmarshal(jsonBytes, &r); err == nil && r.TargetType == 1 {
				for _, id := range r.TargetIDs {
					reportDeltas[id] += delta
				}
			}
		} else if e.Type == redis.EntityTelemetry {
			var t struct {
				PostID       int64 `json:"post_id"`
				DwellTimeMs  int   `json:"dwell_time_ms"`
				IsClicked    bool  `json:"is_clicked"`
				DeepScroll   bool  `json:"deep_scroll"`
				ProfileVisit bool  `json:"profile_visit"`
			}
			if err := json.Unmarshal(jsonBytes, &t); err == nil && t.PostID != 0 {
				// ✅ NOUVEAU : On comptabilise automatiquement une vue classique
				viewDeltas[t.PostID] += 1

				dwellVal := float64(t.DwellTimeMs)
				telemetryDwellSum[t.PostID] += dwellVal
				telemetryDwellSq[t.PostID] += dwellVal * dwellVal

				clicks := 0
				if t.IsClicked {
					clicks++
				}
				if t.DeepScroll {
					clicks++
				}
				if t.ProfileVisit {
					clicks++
				}

				telemetryClicks[t.PostID] += clicks
			}
		} else if e.Type == redis.EntityMessage && e.Action == redis.ActionBuild {
			var p struct {
				MessageID int64          `json:"message_id"`
				Deltas    map[string]int `json:"deltas"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.MessageID != 0 {
				if messageReactionDeltas[p.MessageID] == nil {
					messageReactionDeltas[p.MessageID] = make(map[string]int)
				}
				for emoji, d := range p.Deltas {
					messageReactionDeltas[p.MessageID][emoji] += d
				}
			}
		}
	}

	// Exécution des mises à jour avec gestion d'erreurs
	for id, delta := range likeDeltas {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_post_like($1, $2)", id, delta); err != nil {
			logger.Log.Error().Err(err).Int64("post_id", id).Msg("Erreur mise à jour func_increment_post_like")
		}
	}
	for id, delta := range commentDeltas {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_post_comment($1, $2)", id, delta); err != nil {
			logger.Log.Error().Err(err).Int64("post_id", id).Msg("Erreur mise à jour func_increment_post_comment")
		}
	}
	for id, delta := range viewDeltas {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_post_view($1, $2)", id, delta); err != nil {
			logger.Log.Error().Err(err).Int64("post_id", id).Msg("Erreur mise à jour func_increment_post_view")
		}
	}
	for id, delta := range commentLikeDeltas {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_comment_metrics($1, $2)", id, delta); err != nil {
			logger.Log.Error().Err(err).Int64("comment_id", id).Msg("Erreur mise à jour func_increment_comment_metrics")
		}
	}
	for id, delta := range reportDeltas {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_post_report($1, $2)", id, delta); err != nil {
			logger.Log.Error().Err(err).Int64("post_id", id).Msg("Erreur mise à jour func_increment_post_report")
		}
	}
	for id, sum := range telemetryDwellSum {
		if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT content.func_increment_post_telemetry($1, $2, $3, $4)", id, sum, telemetryDwellSq[id], telemetryClicks[id]); err != nil {
			logger.Log.Error().Err(err).Int64("post_id", id).Msg("Erreur mise à jour func_increment_post_telemetry")
		}
	}
	for msgID, deltas := range messageReactionDeltas {
		validDeltas := make(map[string]int)
		for emoji, d := range deltas {
			if d != 0 {
				validDeltas[emoji] = d
			}
		}

		if len(validDeltas) > 0 {
			deltasJSON, err := json.Marshal(validDeltas)
			if err != nil {
				logger.Log.Error().Err(err).Int64("message_id", msgID).Msg("Erreur Marshal validDeltas")
				continue
			}
			if _, err := postgres.PostgresDB.ExecContext(ctx, "SELECT messaging.func_apply_reaction_delta($1, $2::jsonb)", msgID, string(deltasJSON)); err != nil {
				logger.Log.Error().Err(err).Int64("message_id", msgID).Msg("Erreur mise à jour func_apply_reaction_delta")
			}
		}
	}
}
