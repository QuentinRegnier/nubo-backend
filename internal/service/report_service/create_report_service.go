package report_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// SubmitReport génère le signalement et l'envoie aux workers pour persistance.
func SubmitReport(ctx context.Context, input report_models.CreateReportInput) error {

	now := time.Now().UTC()

	payload := report_models.ReportPayload{
		ID:         pkg.GenerateID(), // Génération du Snowflake ID
		ReporterID: input.UserID,
		TargetType: input.TargetType,
		TargetIDs:  input.TargetIDs,
		Category:   input.Category,
		Reason:     input.Reason,
		State:      variables.ReportStatePending, // 0 par défaut
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// On envoie dans la file d'attente.
	// L'EntityReport n'existe peut-être pas encore dans tes constantes Redis, il faudra l'ajouter !
	// ActionCreate = On crée un nouveau signalement. TargetPostgres = Seulement besoin du L3 !
	return redis.EnqueueDB(ctx, payload.ID, 0, redis.EntityReport, redis.ActionCreate, payload, redis.TargetPostgres)
}
