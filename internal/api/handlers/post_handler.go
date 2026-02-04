package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/gin-gonic/gin"
)

func CreatePostHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	fmt.Printf("✅ Requête Post reçue pour UserID: %d\n", userID)

	// 2. Parsing des données multipart (JSON + Images)
	var input domain.CreatePostInput
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Field 'data' is required"})
		return
	}
	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	// 1. Limiter la quantité (ex: max 10 tags)
	if len(input.Identifiers) > variables.MaxTags && len(input.Hashtags) > variables.MaxTags {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Too many tags (max 10)"})
		return
	}

	// 2. Supprimer les doublons (évite de stocker 10x le même ID)
	input.Identifiers = pkg.SliceUniqueInt64(input.Identifiers)
	input.Hashtags = pkg.SliceUniqueStr(input.Hashtags)

	// 3. Récupération des images (1 à 4 autorisées)
	form, _ := c.MultipartForm()
	files := form.File["media"]
	if len(files) > 4 {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Maximum 4 images allowed"})
		return
	}

	// 4. Appel au service pour la création
	postID, err := service.CreatePost(userID, input, files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Failed to create post: " + err.Error()})
		return
	}

	// 5. Réponse demandée
	c.JSON(http.StatusCreated, domain.CreatePostResponse{
		PostID: postID,
	})
}
