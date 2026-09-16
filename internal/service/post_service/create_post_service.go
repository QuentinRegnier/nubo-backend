package post_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// CreatePost orchestre la publication d'un post.
func CreatePost(ctx context.Context, userID int64, input post_models.CreatePostInput) (int64, error) {
	now := time.Now().UTC()
	postID := pkg.GenerateID()

	// 1. DÉLÉGATION : Activation des médias fantômes
	if err := media_service.ActivateMediaBatch(ctx, input.MediaIDs, userID); err != nil {
		return -1, err
	}

	// 2. DÉLÉGATION : Évaluation dynamique de la priorité
	priorityLevel := 0
	if userLite, err := cache_service.GetUserLite(ctx, userID); err == nil {
		gradeToPriority := map[int]int{
			0: 0, 1: 1, 2: 2, 3: 3, 4: 4,
		}
		if p, ok := gradeToPriority[userLite.Grade]; ok {
			priorityLevel = p
		}
	}

	// 3. ASSEMBLAGE : Création de l'objet Post
	post := post_models.PostPayload{
		ID:                postID,
		UserID:            userID,
		Content:           pkg.CleanStr(input.Content),
		Hashtags:          input.Hashtags,
		IndirectHashtags:  nil, // ✅ NOUVEAU : Initialisation stricte anti-fantôme
		Identifiers:       input.Identifiers,
		MediaIDs:          input.MediaIDs,
		Visibility:        input.Visibility,
		PriorityLevel:     priorityLevel,
		Location:          input.Location,
		CreatedAt:         domain.TimeToMillis(now),
		UpdatedAt:         domain.TimeToMillis(now),
		LikeCount:         0,
		CommentCount:      0,
		ViewCount:         0,
		ReportCount:       0,
		HasMedia:          len(input.MediaIDs) > 0,
		VectorVersion:     1,
		TelemetryDwellSum: 0.0,
		TelemetryDwellSq:  0.0,
		TelemetryClicks:   0,
	}
	// 4. DÉLÉGATION : Vectorisation synchrone du contenu (O(1))
	post.Vector = algorithm_service.ComputeContentVectorFull(post, nil)

	// 5. DÉLÉGATION : Cache Redis (LFU Init) & Timeline
	if err := object_cache_service.SetPostInObjectCache(ctx, post); err != nil {
		return -1, err
	}
	_ = cache_service.AddPostToUserProfile(ctx, userID, postID, float64(now.UnixMilli()))

	// 6. DÉLÉGATION : Persistance Asynchrone (Write-Behind)
	err := redis.EnqueueDB(ctx, postID, 0, redis.EntityPost, redis.ActionCreate, post, redis.TargetAll)

	return postID, err
}
