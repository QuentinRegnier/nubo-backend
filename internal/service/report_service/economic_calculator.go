package report_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/report_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// CalculateEconomicImportance évalue la valeur "financière/rétention" d'une cible signalée en O(1).
// Plus le score est élevé, plus le report doit remonter en haut de la file des modérateurs.
func CalculateEconomicImportance(ctx context.Context, input report_models.CreateReportInput) float64 {
	var totalValue float64 = 0.0

	for _, targetID := range input.TargetIDs {
		switch input.TargetType {

		case 0: // 👤 UTILISATEUR (La valeur d'un compte dépend de son influence)
			// 1. Valeur de l'audience (1 abonné = 1 point)
			followerCount := cache_service.GetFollowerCount(ctx, targetID)
			totalValue += float64(followerCount)

			// 2. Valeur du statut (Les VIP coûtent cher s'ils partent ou dérapent)
			if userLite, err := cache_service.GetUserLite(ctx, targetID); err == nil {
				switch userLite.Grade {
				case 1: // Certifié
					totalValue += 5000.0
				case 2: // Partenaire (Monétisé)
					totalValue += 20000.0
				case 3, 4: // Staff / Admin
					totalValue += 50000.0
				}
			}

		case 1: // 📝 PUBLICATION (La valeur d'un post dépend de la rétention publicitaire)
			// On tape dans le L1 (Object Cache LFU) pour avoir les compteurs chauds
			if post, err := object_cache_service.GetPostFromObjectCache(ctx, targetID); err == nil {
				// Formule monétaire fictive (ajustable) :
				// - 1 Vue = 0.1 pt
				// - 1 Like = 2 pts
				// - 1 Commentaire = 5 pts (Génère du retour sur l'app)
				// - 1 seconde de Dwell Time = 0.5 pt (Temps de cerveau publicitaire)

				viewsValue := float64(post.ViewCount) * 0.1
				likesValue := float64(post.LikeCount) * 2.0
				commentsValue := float64(post.CommentCount) * 5.0

				// Le Dwell Sum est stocké en millisecondes, on le convertit en secondes
				dwellSeconds := post.TelemetryDwellSum / 1000.0
				dwellValue := dwellSeconds * 0.5

				totalValue += viewsValue + likesValue + commentsValue + dwellValue
			}

		case 2: // 💬 COMMENTAIRE
			if comment, err := object_cache_service.GetCommentFromObjectCache(ctx, targetID); err == nil {
				totalValue += float64(comment.LikeCount) * 1.5
			}
		}
	}

	return totalValue
}
