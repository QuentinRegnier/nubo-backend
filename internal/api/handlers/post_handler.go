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

// CreatePostHandler godoc
// @Summary      Créer une nouvelle publication
// @Description  Crée un post avec du contenu texte, des hashtags, des mentions d'utilisateurs et entre 1 et 4 images.
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `Field 'data' is required` : Le champ texte 'data' contenant le JSON est manquant.
// @Description  * `Invalid JSON: ...` : Le format JSON dans le champ 'data' est incorrect.
// @Description  * `Too many tags (max 10)` : Le nombre de hashtags ou d'utilisateurs tagués dépasse 10.
// @Description  * `Maximum 4 images allowed` : Vous avez tenté d'envoyer plus de 4 fichiers média.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * `Utilisateur non identifié` : Le userID n'a pas pu être extrait du token JWT ou contexte manquant.
// @Description  * `Signature HMAC invalide` : (Géré par le middleware) La signature ne correspond pas au contenu.
// @Description
// @Description  ⚫ **500 Internal Server Error (Serveur) :**
// @Description  * `Failed to create post: ...` : Erreur lors de l'upload MinIO ou de l'insertion dans la file d'attente Redis (Queue).
// @Tags         posts
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        media         formData file   false "Images du post (1 à 4 fichiers)"
// @Param        data          formData string true  "Données JSON (domain.CreatePostInput)"
// @Success      201  {object}  domain.CreatePostResponse
// @Failure      400  {object}  domain.ErrorResponse "Données invalides ou trop de fichiers"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou signature HMAC corrompue"
// @Failure      500  {object}  domain.ErrorResponse "Erreur interne de persistance"
// @Router       /post [post]
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
