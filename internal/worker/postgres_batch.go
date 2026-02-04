package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/lib/pq"
)

// GenerateCopyQuery crée une requête COPY qui respecte les schémas (ex: auth.users)
func GenerateCopyQuery(fullTableName string, columns []string) string {
	// On sépare le schéma de la table si un point est présent
	quotedTableName := ""
	if strings.Contains(fullTableName, ".") {
		parts := strings.SplitN(fullTableName, ".", 2)
		quotedTableName = pq.QuoteIdentifier(parts[0]) + "." + pq.QuoteIdentifier(parts[1])
	} else {
		quotedTableName = pq.QuoteIdentifier(fullTableName)
	}

	// On quote les colonnes
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

	// On définit l'ordre de priorité pour respecter les Clés Étrangères
	executionOrder := []redis.EntityType{
		redis.EntityUser,         // 1. Les Parents d'abord
		redis.EntityUserSettings, // 2. Les paramètres utilisateur
		redis.EntitySession,      // 3. Les sessions (dépendent du User)
		redis.EntityRelation,     // 4. Les relations entre utilisateurs
		redis.EntityPost,         // 5. Le contenu
		redis.EntityComment,      // 6. Les commentaires
		redis.EntityLike,         // 7. Les likes
		redis.EntityMedia,        // 8. Les médias
		redis.EntityConversation, // 9. Les conversations
		redis.EntityMembers,      // 10. Les membres de conversation
		redis.EntityMessage,      // 11. Les messages
		// ... les autres ...
	}

	// 1. Exécution ordonnée
	for _, entityType := range executionOrder {
		if evts, exists := grouped[entityType]; exists {
			processEntityEvents(ctx, entityType, evts)
			delete(grouped, entityType) // On retire pour ne pas le refaire
		}
	}

	// 2. Exécution du reste (ce qui n'est pas dans la liste prioritaire)
	for entityType, evts := range grouped {
		processEntityEvents(ctx, entityType, evts)
	}
}

// processEntityEvents : Fonction helper pour éviter de dupliquer le code dans les boucles
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
		bulkInsertPostgres(ctx, entityType, inserts)
	}
	if len(updates) > 0 {
		bulkUpdatePostgres(ctx, entityType, updates)
	}
	if len(deletes) > 0 {
		bulkDeletePostgres(ctx, entityType, deletes)
	}
}

// --- 1. BULK INSERT (Via lib/pq CopyIn) ---
func bulkInsertPostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) {
	mapper := GetMapper(entity)
	if mapper == nil {
		log.Printf("❌ Pas de mapper Postgres pour %s", entity)
		return
	}

	// On utilise une transaction pour le COPY
	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("❌ Erreur BeginTx Insert: %v", err)
		return
	}

	committed := false
	defer func() {
		if !committed {
			err := tx.Rollback()
			if err != nil && err != sql.ErrTxDone {
				log.Printf("⚠️ Erreur lors du Rollback: %v", err)
			}
		}
	}()

	// CORRECTION ICI : On n'utilise plus pq.CopyIn directement car il gère mal les schémas "auth.users"
	copyQuery := GenerateCopyQuery(mapper.TableName(), mapper.Columns())

	// Préparation du COPY
	stmt, err := tx.Prepare(copyQuery)
	if err != nil {
		log.Printf("❌ Erreur Prepare CopyIn: %v", err)
		return
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// On pousse les données
	for _, e := range events {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			log.Printf("❌ Erreur mapping payload: %v", err)
			continue // On passe à l'événement suivant
		}
		_, err = stmt.Exec(row...)
		if err != nil {
			log.Printf("❌ Erreur Exec CopyIn: %v", err)
			return
		}
	}

	// On ferme le statement pour flusher les données
	if _, err := stmt.Exec(); err != nil {
		log.Printf("❌ Erreur Flush CopyIn: %v", err)
		_ = stmt.Close() // On ferme quand même
		return
	}

	if err := stmt.Close(); err != nil {
		log.Printf("❌ Erreur closing statement: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("❌ Erreur Commit Insert: %v", err)
	}

	committed = true
}

// --- 2. BULK UPDATE (Temp Table + COPY) ---
func bulkUpdatePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) {
	mapper := GetMapper(entity)
	if mapper == nil {
		return
	}

	tx, err := postgres.PostgresDB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("❌ Erreur BeginTx Update: %v", err)
		return
	}

	committed := false
	defer func() {
		if !committed {
			err := tx.Rollback()
			if err != nil && err != sql.ErrTxDone {
				log.Printf("⚠️ Erreur lors du Rollback: %v", err)
			}
		}
	}()

	// A. Création table temporaire (copie structure)
	// On utilise time.Now().UnixNano() pour un nom unique par batch
	safeTableName := strings.ReplaceAll(mapper.TableName(), ".", "_")
	tempTable := fmt.Sprintf("tmp_%s_%d", safeTableName, time.Now().UnixNano())

	// Note: ON COMMIT DROP assure que la table disparait à la fin de la transaction
	queryCreateTable := fmt.Sprintf("CREATE TEMP TABLE %s (LIKE %s INCLUDING ALL) ON COMMIT DROP", tempTable, mapper.TableName())
	if _, err := tx.ExecContext(ctx, queryCreateTable); err != nil {
		log.Printf("❌ Erreur création Temp Table (%s): %v", tempTable, err)
		return
	}

	// B. Remplissage table temporaire (COPY)
	stmt, err := tx.Prepare(pq.CopyIn(tempTable, mapper.Columns()...))
	if err != nil {
		log.Printf("❌ Erreur Prepare CopyIn Temp: %v", err)
		return
	}

	for _, e := range events {
		row, err := mapper.ToRow(e.Payload)
		if err != nil {
			log.Printf("❌ Erreur mapping payload pour update: %v", err)
			continue
		}
		if _, err := stmt.Exec(row...); err != nil {
			log.Printf("❌ Erreur Exec CopyIn Temp: %v", err)
			_ = stmt.Close()
			return
		}
	}

	if _, err := stmt.Exec(); err != nil {
		log.Printf("❌ Erreur Flush CopyIn Temp: %v", err)
		_ = stmt.Close() // On ignore ici car on gère déjà l'erreur Exec
		return
	}

	if err := stmt.Close(); err != nil {
		log.Printf("❌ Erreur closing statement temp: %v", err)
		return
	}

	// C. Merge (UPDATE FROM)
	queryUpdate := mapper.BuildUpdateQuery(tempTable)
	if _, err := tx.ExecContext(ctx, queryUpdate); err != nil {
		log.Printf("❌ Erreur Merge Update: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("❌ Erreur Commit Update: %v", err)
	}

	committed = true
}

// --- 3. BULK DELETE (WHERE ID = ANY(...)) ---
func bulkDeletePostgres(ctx context.Context, entity redis.EntityType, events []redis.AsyncEvent) {
	mapper := GetMapper(entity)
	if mapper == nil {
		return
	}

	ids := make([]int64, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	// Utilisation de pq.Array pour passer la liste d'IDs proprement
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", mapper.TableName())

	_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(ids))
	if err != nil {
		log.Printf("❌ Erreur Bulk Delete: %v", err)
	}
}
