package report_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/numan-backend/internal/domain"
	"github.com/QuentinRegnier/numan-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
	"github.com/QuentinRegnier/numan-backend/internal/variables"
)

// ############################################################################
// # SERVICE : CRÉATION DE SIGNALEMENT
// ############################################################################

// SubmitReport génère le signalement, évalue son urgence économique en RAM (O(1)),
// et l'envoie aux workers pour persistance dans PostgreSQL.
func SubmitReport(ctx context.Context, callerID int64, input report_models.CreateReportInput) error {

	currentTime := time.Now().UTC()

	// ── ÉTAPE 1 : CALCUL DU SCORE D'URGENCE (O(1) EN RAM) ───────────────────
	err := pkg.ListLimitVerif(input.TargetIDs, variables.MaxReportTargets)
	if err != nil {
		return err
	}
	economicValueScore := calculateEconomicImportance(ctx, input)

	// ── ÉTAPE 2 : CONSTRUCTION DU PAYLOAD DE SIGNALEMENT ────────────────────

	reportPayload := report_models.ReportPayload{
		ID:         pkg.GenerateID(),
		ReporterID: callerID,
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
		numan_log.Error(ctx).Err(errQueue).Int64("reporter_id", callerID).Msg("Échec du Write-Behind lors de la création d'un signalement")
		return numan_error.NewInternal()
	}

	return nil
}
