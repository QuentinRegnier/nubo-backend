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
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// GetProfile est le Hub qui orchestre la récupération de toutes les strates d'un profil.
func GetProfile(ctx context.Context, callerID int64, input profile_models.GetProfileInput) (profile_models.GetProfileOutput, error) {
	targetID := input.TargetID
	if targetID == 0 {
		targetID = callerID
	}

	output := profile_models.GetProfileOutput{
		LikedPostIDs: make([]int64, 0),
		SavedPostIDs: make([]int64, 0),
		Posts:        make([]post_models.GetPostOutput, 0),
	}

	isSelf := callerID == targetID

	// 1. MATRICE DE RELATIONS
	if !isSelf {
		output.RelationViewerToTarget = cache_service.RelationValue(ctx, targetID, callerID)
		output.RelationTargetToViewer = cache_service.RelationValue(ctx, callerID, targetID)

		if output.RelationTargetToViewer == -1 {
			return profile_models.GetProfileOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Utilisateur introuvable.", nil)
		}
	}

	// 2. RÉCUPÉRATION DES PARAMÈTRES ET APPLICATION DES VERROUS
	settings, errSet := object_cache_service.GetUserSettingsCascade(ctx, targetID)

	if errSet == nil && !isSelf {
		// ✅ APPLICATION: Profile Visibility
		canViewProfile := false
		switch settings.Privacy.ProfileVisibility {
		case 0:
			canViewProfile = true
		case 1:
			canViewProfile = (output.RelationViewerToTarget >= 1)
		case 2:
			canViewProfile = (output.RelationViewerToTarget == 2)
		}
		if !canViewProfile {
			return profile_models.GetProfileOutput{}, nubo_error.NewForbidden("PROFILE_PRIVATE", "Ce profil est privé.", nil)
		}
	}

	// 3. IDENTITÉ DE L'UTILISATEUR
	user, err := mongo.MongoLoadUser(targetID, "", "", "")
	if err != nil || user.ID == 0 {
		user, err = postgres.FuncLoadUser(targetID, "", "", "")
		if err != nil || user.ID == 0 {
			return profile_models.GetProfileOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Utilisateur introuvable.", err)
		}
		go func(u auth_models.UserPayload) {
			_ = redis.EnqueueDB(context.Background(), u.ID, 0, redis.EntityUser, redis.ActionUpdate, u, redis.TargetMongo)
		}(user)
	}

	// ✅ APPLICATION: Show Location
	if errSet == nil && !settings.Privacy.ShowLocation && !isSelf {
		user.Location = ""
	}

	// ✅ APPLICATION: Show Online Status
	isOnline := cache_service.IsUserOnline(ctx, user.ID)
	if errSet == nil && !settings.Privacy.ShowOnlineStatus && !isSelf {
		isOnline = false
	}

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
		IsOnline:  isOnline,
	}

	// 4. AVATAR
	if user.ProfilePictureID > 0 {
		if view, errMedia := media_service.GenerateMediaViewCascade(ctx, user.ProfilePictureID, targetID, 0, callerID); errMedia == nil {
			output.Avatar = view
		}
	}

	// 5. CONVERSATION DIRECTE
	if !isSelf {
		convID, errCache := cache_service.GetDirectConversationCache(ctx, callerID, targetID)
		if errCache == nil && convID > 0 {
			output.DirectConversationID = convID
		} else {
			c, errPg := postgres.FuncGetDirectConversation(ctx, callerID, targetID)
			if errPg == nil && c.ID > 0 {
				output.DirectConversationID = c.ID
			}
		}
	}

	// 6. CHARGEMENT DU BATCH DE POSTS
	postInput := post_models.GetUserPostsInput{
		TargetUserID: targetID,
		Limit:        input.Limit,
		Offset:       input.Offset,
		Force:        false,
	}
	postsOutput := post_service.GetUserPosts(ctx, postInput)

	if len(postsOutput) > 0 {
		output.Posts = postsOutput

		batchPostIDs := make([]int64, 0, len(postsOutput))
		for _, p := range postsOutput {
			batchPostIDs = append(batchPostIDs, p.PostID)
		}

		for _, pID := range batchPostIDs {
			likes, _ := postgres.FuncLoadLikes(ctx, 0, pID, callerID, 1, 0)
			if len(likes) > 0 {
				output.LikedPostIDs = append(output.LikedPostIDs, pID)
			}
		}

		recentSaved, _ := postgres.FuncLoadSavedPosts(ctx, callerID, 500, 0)
		savedMap := make(map[int64]bool)
		for _, s := range recentSaved {
			savedMap[s.PostID] = true
		}

		for _, pID := range batchPostIDs {
			if savedMap[pID] {
				output.SavedPostIDs = append(output.SavedPostIDs, pID)
			}
		}
	}

	return output, nil
}
