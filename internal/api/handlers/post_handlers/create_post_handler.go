package post_handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// CreatePostHandler godoc
// @Summary      Créer une nouvelle publication
// @Description  Crée un post_service avec du contenu texte, des hashtags, des mentions d'utilisateurs et entre 1 et 4 images.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Field 'data' is required` : Le champ texte 'data' contenant le JSON est manquant.
// @Description  * `Invalid JSON: ...` : Le format JSON dans le champ 'data' est incorrect.
// @Description  * `Validation failed: ...` : (Géré par pkg.ValidateStruct) Le contenu, les hashtags ou la visibilité ne respectent pas les limites.
// @Description  * `Maximum 4 images allowed` : Vous avez tenté d'envoyer plus de 4 fichiers média.
// @Description  * `Empty post_service` : Impossible de publier un post_service sans texte et sans média.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Utilisateur non identifié` : Le userID n'a pas pu être extrait du token JWT ou contexte manquant.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * `Failed to create post_service: ...` : Erreur lors de l'upload MinIO ou de l'insertion dans la file d'attente Redis (Queue).
// @Tags         posts
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        media         formData file   false "Images du post_service (1 à 4 fichiers)"
// @Param        data          formData string true  "Données JSON (domain.CreatePostInput)"
// @Success      201  {object}  domain.CreatePostResponse
// @Failure      400  {object}  domain.ErrorResponse "Données invalides ou trop de fichiers"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne de persistance"
// @Router       /posts [post_service]
func CreatePostHandler(c *gin.Context) {
	// 1. Authentification via le contexte Gin (Middleware JWT)
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	fmt.Printf("✅ Requête Post reçue pour UserID: %d\n", userID)

	// 2. Parsing des données multipart (JSON + Images)
	var input post_models.CreatePostInput
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Field 'data' is required"})
		return
	}

	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	// 3. 🛡 BOUCLIER STATIQUE : Validation O(1)
	if err := pkg.ValidateStruct(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Validation failed: " + err.Error()})
		return
	}

	// 4. Nettoyage de la donnée (Sécurité contre l'empoisonnement de BDD)
	// Supprimer les doublons (évite de stocker 10x le même ID utilisateur ou hashtag)
	input.Identifiers = pkg.SliceUniqueInt64(input.Identifiers)
	input.Hashtags = pkg.SliceUniqueStr(input.Hashtags)

	// 5. Récupération des images (1 à 4 autorisées)
	form, _ := c.MultipartForm()
	files := form.File["media"]
	if len(files) > 4 {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Maximum 4 images allowed"})
		return
	}

	// Prévention stricte des "Posts Fantômes"
	if input.Content == "" && len(files) == 0 {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Empty post_service: un texte ou un média est requis"})
		return
	}

	// 6. Appel au service métier pour la création (et Fan-Out asynchrone)
	postID, err := post_service.CreatePost(userID, input, files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Failed to create post_service: " + err.Error()})
		return
	}

	// 7. Réponse HTTP 201
	c.JSON(http.StatusCreated, post_models.CreatePostResponse{
		PostID: postID,
	})
}
