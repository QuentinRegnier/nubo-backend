package post_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"
)

// CreatePostHandler godoc
// @Summary      Créer une nouvelle publication
// @Description  Crée un post avec du contenu texte, des hashtags, des mentions et de 0 à 4 IDs d'images préalablement uploadées (Out-of-Band).
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body post_models.CreatePostInput true "Données du post"
// @Success      201  {object} post_models.CreatePostResponse
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /posts [post]
func CreatePostHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input post_models.CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Invalid JSON ou validation échouée.", err))
		return
	}

	input.Identifiers = pkg.SliceUniqueInt64(input.Identifiers)
	input.Hashtags = pkg.SliceUniqueStr(input.Hashtags)
	input.Content = pkg.CleanStr(input.Content)
	input.Location = pkg.CleanStr(input.Location)

	if input.Content == "" && len(input.MediaIDs) == 0 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("EMPTY_POST", "Un texte ou un média est requis.", nil))
		return
	}

	if len(input.MediaIDs) > 4 {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("TOO_MANY_MEDIA", "Maximum 4 images allowed.", nil))
		return
	}

	postID, err := post_service.CreatePost(c.Request.Context(), userID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, post_models.CreatePostResponse{
		PostID: postID,
	})
}
