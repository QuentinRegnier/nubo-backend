package profile_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/profile_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// GetProfile est le Hub qui orchestre la récupération de toutes les strates d'un profil.
func GetProfile(ctx context.Context, callerID int64, input profile_models.GetProfileInput) (profile_models.GetProfileOutput, error) {
	targetID := input.TargetID
	if targetID == 0 {
		targetID = callerID // Si pas de cible, on charge notre propre profil
	}

	output := profile_models.GetProfileOutput{
		LikedPostIDs: make([]int64, 0),
		SavedPostIDs: make([]int64, 0),
		Posts:        make([]post_models.GetPostOutput, 0),
	}

	isSelf := callerID == targetID

	// ========================================================================
	// 1. MATRICE DE RELATIONS (O(1) en RAM)
	// ========================================================================
	if !isSelf {
		output.RelationViewerToTarget = cache_service.RelationValue(ctx, targetID, callerID)
		output.RelationTargetToViewer = cache_service.RelationValue(ctx, callerID, targetID)

		// Bouclier de sécurité : si la cible nous a bloqués, on simule une 404 (Shadow ban)
		if output.RelationTargetToViewer == -1 {
			return profile_models.GetProfileOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Utilisateur introuvable.", nil)
		}
		// Si NOUS l'avons bloqué (ViewerToTarget == -1), on laisse passer pour qu'il puisse voir le profil et cliquer sur "Report" ou "Débloquer".
	}

	// ========================================================================
	// 2. IDENTITÉ DE L'UTILISATEUR (Cascade L2 -> L3)
	// ========================================================================
	// On a besoin de la bio, location, school, etc., donc le Speed Cache L1 ne suffit pas.
	user, err := mongo.MongoLoadUser(targetID, "", "", "")
	if err != nil || user.ID == 0 {
		user, err = postgres.FuncLoadUser(targetID, "", "", "")
		if err != nil || user.ID == 0 {
			return profile_models.GetProfileOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Utilisateur introuvable.", err)
		}
		// Promotion L3 -> L2 asynchrone (Auto-Guérison)
		go func(u auth_models.UserPayload) {
			_ = redis.EnqueueDB(context.Background(), u.ID, 0, redis.EntityUser, redis.ActionUpdate, u, redis.TargetMongo)
		}(user)
	}

	// Mapping sécurisé des champs utiles
	output.User = auth_models.UserProfileView{
		ID:        user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Birthdate: user.Birthdate,
		Sex:       user.Sex,
		Bio:       user.Bio,
		Grade:     user.Grade,
		Location:  user.Location,
		School:    user.School,
		Work:      user.Work,
		Badges:    user.Badges,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		IsOnline:  cache_service.IsUserOnline(ctx, user.ID), // O(1) L1 Call
	}

	// ========================================================================
	// 3. AVATAR (Génération du lien HMAC signé)
	// ========================================================================
	if user.ProfilePictureID > 0 {
		if view, errMedia := media_service.GenerateMediaViewCascade(ctx, user.ProfilePictureID, targetID, 0, callerID); errMedia == nil {
			output.Avatar = view
		}
	}

	// ========================================================================
	// 4. CONVERSATION DIRECTE (O(log N) RAM L1 + Fallback L2/L3)
	// ========================================================================
	if !isSelf {
		// On cherche juste l'information "Est-ce qu'on a déjà un MP actif ?"
		convID, errCache := cache_service.GetDirectConversationCache(ctx, callerID, targetID)
		if errCache == nil && convID > 0 {
			output.DirectConversationID = convID
		} else {
			// Fallback BDD (Pur DDD)
			c, errPg := postgres.FuncGetDirectConversation(ctx, callerID, targetID)
			if errPg == nil && c.ID > 0 {
				output.DirectConversationID = c.ID
			}
		}
	}

	// ========================================================================
	// 5. CHARGEMENT DU BATCH DE POSTS
	// ========================================================================
	postInput := post_models.GetUserPostsInput{
		TargetUserID: targetID,
		Limit:        input.Limit,
		Offset:       input.Offset,
		Force:        false,
	}
	// Utilisation du service existant pour récupérer les publications
	postsOutput := post_service.GetUserPosts(ctx, postInput)
	if len(postsOutput) > 0 {
		output.Posts = postsOutput

		// ========================================================================
		// 6. INTERACTIONS (Likes & Saved) APPARTENANT AU CALLER
		// ========================================================================
		// Technique anti-N+1: On récupère les likes/saves et on croise avec le batch

		// A. Map des posts retournés
		batchPostIDs := make([]int64, 0, len(postsOutput))
		for _, p := range postsOutput {
			batchPostIDs = append(batchPostIDs, p.PostID)
		}

		// B. Recherche des Likes (Via repository existant Postgres)
		for _, pID := range batchPostIDs {
			likes, _ := postgres.FuncLoadLikes(ctx, 0, pID, callerID, 1, 0)
			if len(likes) > 0 {
				output.LikedPostIDs = append(output.LikedPostIDs, pID)
			}
		}

		// C. Recherche des Sauvegardes
		// Étant donné qu'on ne doit pas injecter de SQL et que FuncLoadSavedPosts prend l'user_id,
		// on récupère le lot le plus récent de l'utilisateur (Max 500) et on crée une map d'intersection en RAM.
		recentSaved, _ := postgres.FuncLoadSavedPosts(ctx, callerID, 500, 0)
		savedMap := make(map[int64]bool)
		for _, s := range recentSaved {
			savedMap[s.PostID] = true
		}

		// On vérifie le batch contre la map
		for _, pID := range batchPostIDs {
			if savedMap[pID] {
				output.SavedPostIDs = append(output.SavedPostIDs, pID)
			}
		}
	}

	return output, nil
}
