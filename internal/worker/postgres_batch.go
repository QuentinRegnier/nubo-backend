package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
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
		if evts, exists := grouped[entityType]; exists {
			processEntityEvents(ctx, entityType, evts)
			delete(grouped, entityType)
		}
	}

	// 2. Exécution du reste
	for entityType, evts := range grouped {
		processEntityEvents(ctx, entityType, evts)
	}
}

// processEntityEvents gère le Fast Path et déclenche le Slow Path en cas d'erreur.
func processEntityEvents(ctx context.Context, entityType redis.EntityType, evts []redis.AsyncEvent) {
	var inserts, updates, deletes []redis.AsyncEvent

	for _, e := range evts {
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
			log.Printf("⚠️ Fast Path Insert failed pour %s: %v. Déclenchement Dichotomie...", entityType, err)
			slowPathDichotomy(ctx, entityType, redis.ActionCreate, inserts)
		}
	}
	if len(updates) > 0 {
		if err := bulkUpdatePostgres(ctx, entityType, updates); err != nil {
			log.Printf("⚠️ Fast Path Update failed pour %s: %v. Déclenchement Dichotomie...", entityType, err)
			slowPathDichotomy(ctx, entityType, redis.ActionUpdate, updates)
		}
	}
	if len(deletes) > 0 {
		if err := bulkDeletePostgres(ctx, entityType, deletes); err != nil {
			log.Printf("⚠️ Fast Path Delete failed pour %s: %v. Déclenchement Dichotomie...", entityType, err)
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
		"error":  dbErr.Error(),
		"time":   time.Now().Format(time.RFC3339),
		"entity": entity,
		"action": action,
		"event":  event,
	}

	bytes, err := json.Marshal(dlqPayload)
	if err == nil {
		redisgo.Rdb.LPush(ctx, "dlq:postgres_errors", bytes)
		log.Printf("🚨 [DLQ] Événement isolé et mis en quarantaine : %s %s (ID: %d)", entity, action, event.ID)
	}
}

// ============================================================================
// 1. BULK INSERT (Via lib/pq CopyIn)
// ============================================================================
func bulkInsertPostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	mapper := GetMapper(entity)
	if mapper == nil {
		return fmt.Errorf("pas de mapper Postgres pour %s", entity)
	}

	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("BeginTx Insert: %w", err)
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
		return fmt.Errorf("prepare CopyIn: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			log.Printf("⚠️ Erreur fermeture statement CopyIn: %v", err)
		}
	}(stmt) // Gère la fermeture proprement.

	for _, e := range events {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			return fmt.Errorf("mapping payload: %w", err)
		}
		if _, err = stmt.Exec(row...); err != nil {
			return fmt.Errorf("exec CopyIn: %w", err)
		}
	}

	// Flush du COPY
	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("flush CopyIn: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Insert: %w", err)
	}

	committed = true
	return nil
}

// ============================================================================
// 2. BULK UPDATE (Temp Table + COPY)
// ============================================================================
func bulkUpdatePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	mapper := GetMapper(entity)
	if mapper == nil {
		return fmt.Errorf("pas de mapper Postgres pour %s", entity)
	}

	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("BeginTx Update: %w", err)
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
		return fmt.Errorf("création Temp Table: %w", err)
	}

	stmt, err := tx.Prepare(pq.CopyIn(tempTable, mapper.Columns()...))
	if err != nil {
		return fmt.Errorf("prepare CopyIn Temp: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			log.Printf("⚠️ Erreur fermeture statement CopyIn Temp: %v", err)
		}
	}(stmt)

	for _, e := range events {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			return fmt.Errorf("mapping payload update: %w", err)
		}
		if _, err := stmt.Exec(row...); err != nil {
			return fmt.Errorf("exec CopyIn Temp: %w", err)
		}
	}

	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("flush CopyIn Temp: %w", err)
	}

	queryUpdate := mapper.BuildUpdateQuery(tempTable)
	if _, err := tx.ExecContext(ctx, queryUpdate); err != nil {
		return fmt.Errorf("merge Update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Update: %w", err)
	}

	committed = true
	return nil
}

// ============================================================================
// 3. BULK DELETE (WHERE ID = ANY(...))
// ============================================================================
func bulkDeletePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) error {
	mapper := GetMapper(entity)
	if mapper == nil {
		return fmt.Errorf("pas de mapper Postgres pour %s", entity)
	}

	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", mapper.TableName())

	_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("erreur Bulk Delete: %w", err)
	}

	return nil
}
