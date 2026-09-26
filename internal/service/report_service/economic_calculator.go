package report_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # UTILITAIRE : CALCULATEUR D'IMPORTANCE ÉCONOMIQUE
// ############################################################################

// CalculateEconomicImportance évalue la valeur "financière/rétention" d'une cible signalée en O(1).
// Plus le score est élevé, plus le signalement doit remonter en haut de la file des modérateurs.
func CalculateEconomicImportance(ctx context.Context, input report_models.CreateReportInput) float64 {
	var totalEconomicValue = 0.0

	for _, targetID := range input.TargetIDs {
		switch input.TargetType {

		case variables.ReportTargetUser:
			// 1. Valeur de l'audience
			followerCount := cache_service.GetFollowerCount(ctx, targetID)
			totalEconomicValue += float64(followerCount) * variables.ReportScoreFollowerMultiplier

			// 2. Valeur du statut (Les VIP coûtent cher s'ils partent ou dérapent)
			if targetUserLite, errCache := cache_service.GetUserLite(ctx, targetID); errCache == nil {
				switch targetUserLite.Grade {
				case 1: // Certifié
					totalEconomicValue += variables.ReportScoreCertifiedBonus
				case 2: // Partenaire (Monétisé)
					totalEconomicValue += variables.ReportScorePartnerBonus
				case 3, 4: // Staff / Admin
					totalEconomicValue += variables.ReportScoreStaffBonus
				}
			}

		case variables.ReportTargetPost:
			// Valeur de rétention publicitaire (Object Cache LFU pour les compteurs chauds)
			if postPayload, errCache := object_cache_service.GetPostFromObjectCache(ctx, targetID); errCache == nil {

				viewsValue := float64(postPayload.ViewCount) * variables.ReportScoreViewMultiplier
				likesValue := float64(postPayload.LikeCount) * variables.ReportScoreLikeMultiplier
				commentsValue := float64(postPayload.CommentCount) * variables.ReportScoreCommentMultiplier

				// Le Dwell Sum est stocké en millisecondes, on le convertit en secondes
				dwellSeconds := postPayload.TelemetryDwellSum / 1000.0
				dwellValue := dwellSeconds * variables.ReportScoreDwellMultiplier

				totalEconomicValue += viewsValue + likesValue + commentsValue + dwellValue
			}

		case variables.ReportTargetComment:
			// Valeur d'engagement d'un commentaire
			if commentPayload, errCache := object_cache_service.GetCommentFromObjectCache(ctx, targetID); errCache == nil {
				totalEconomicValue += float64(commentPayload.LikeCount) * variables.ReportScoreCommentLikeMultiplier
			}
		}
	}

	return totalEconomicValue
}
