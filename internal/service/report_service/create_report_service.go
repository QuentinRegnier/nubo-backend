package report_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// SubmitReport génère le signalement et l'envoie aux workers pour persistance.
func SubmitReport(ctx context.Context, input report_models.CreateReportInput) error {

	now := time.Now().UTC()

	// 1. Calcul ultra-rapide en RAM (zéro latence DB)
	economicValue := CalculateEconomicImportance(ctx, input)

	reportPayload := report_models.ReportPayload{
		ID:         pkg.GenerateID(),
		ReporterID: input.UserID,
		TargetType: input.TargetType,
		TargetIDs:  input.TargetIDs,
		Category:   input.Category,
		Reason:     input.Reason,
		Rationale:  "",
		State:      variables.ReportStatePending,
		Importance: economicValue, // ✅ Injection du score ici
		CreatedAt:  domain.TimeToMillis(now),
		UpdatedAt:  domain.TimeToMillis(now),
	}

	// 2. Envoi asynchrone
	return redis.EnqueueDB(ctx, reportPayload.ID, 0, redis.EntityReport, redis.ActionCreate, reportPayload, redis.TargetPostgres)
}
