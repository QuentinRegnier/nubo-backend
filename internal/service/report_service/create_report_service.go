package report_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : CRÉATION DE SIGNALEMENT
// ############################################################################

// SubmitReport génère le signalement, évalue son urgence économique en RAM (O(1)),
// et l'envoie aux workers pour persistance dans PostgreSQL.
func SubmitReport(ctx context.Context, input report_models.CreateReportInput) error {

	currentTime := time.Now().UTC()

	// ── ÉTAPE 1 : CALCUL DU SCORE D'URGENCE (O(1) EN RAM) ───────────────────

	economicValueScore := CalculateEconomicImportance(ctx, input)

	// ── ÉTAPE 2 : CONSTRUCTION DU PAYLOAD DE SIGNALEMENT ────────────────────

	reportPayload := report_models.ReportPayload{
		ID:         pkg.GenerateID(),
		ReporterID: input.UserID,
		TargetType: input.TargetType,
		TargetIDs:  input.TargetIDs,
		Category:   input.Category,
		Reason:     input.Reason,
		Rationale:  "", // Champ réservé pour les notes et décisions des modérateurs
		State:      variables.ReportStatePending,
		Importance: economicValueScore,
		CreatedAt:  domain.TimeToMillis(currentTime),
		UpdatedAt:  domain.TimeToMillis(currentTime),
	}

	// ── ÉTAPE 3 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────

	errQueue := redis.EnqueueDB(ctx, reportPayload.ID, 0, redis.EntityReport, redis.ActionCreate, reportPayload, redis.TargetPostgres)

	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("reporter_id", input.UserID).Msg("Échec du Write-Behind lors de la création d'un signalement")
		return nubo_error.NewInternal()
	}

	return nil
}
