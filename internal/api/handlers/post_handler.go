package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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

// BatchViewInput définit la structure attendue : {"post_ids": [1, 2, 3]}
type BatchViewInput struct {
	PostIDs []int64 `json:"post_ids" binding:"required"`
}

// RegisterBatchViewsHandler godoc
// @Summary      Enregistrer des vues en lot (Batching)
// @Description  Permet au client d'envoyer un tableau d'IDs de posts vus toutes les X secondes pour soulager le réseau.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        data body BatchViewInput true "Tableau des IDs des posts vus"
// @Success      200  {object}  domain.SuccessResponse
// @Failure      400  {object}  domain.ErrorResponse "JSON invalide ou tableau trop grand"
// @Router       /views/batch [post]
func RegisterBatchViewsHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	var input BatchViewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	// Sécurité anti-spam : on limite la taille du lot à 100 vues maximum par requête
	// (Personne ne scrolle plus de 100 posts en 10 secondes)
	if len(input.PostIDs) > 100 {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Trop d'IDs dans le lot (max 100)"})
		return
	}

	// On boucle sur les IDs pour les envoyer dans le Buffer en RAM (Temps: 0.01ms)
	for _, postID := range input.PostIDs {
		// Nettoyage anti-doublon direct : s'assurer qu'un ID n'est pas à 0
		if postID > 0 {
			service.RegisterView(userID, postID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vues enregistrées avec succès"})
}

// GetUserPostsHandler récupère la chronologie des posts d'un utilisateur (Profil).
// @Summary      Récupérer les posts d'un utilisateur
// @Description  Récupère les publications d'un profil spécifique via le cache hybride (ZSET -> Mongo -> Postgres).
// @Tags         posts
// @Param        id     path    int     true  "ID de l'utilisateur"
// @Param        offset query   int     false "Offset de pagination"
// @Param        limit  query   int     false "Nombre de posts (max 50)"
// @Router       /users/{id}/posts [get]
func GetUserPostsHandler(c *gin.Context) {
	// 1. Parsing de l'ID cible (Snowflake int64)
	targetUserID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "ID utilisateur invalide"})
		return
	}

	// 2. Pagination avec protection matérielle
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 64)

	if limit > 50 {
		limit = 50
	}

	// 3. Appel au service d'hydratation hybride
	posts, err := service.GetUserProfilePosts(c.Request.Context(), targetUserID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur lors de la récupération des posts"})
		return
	}

	// 4. On garantit un tableau vide [] au lieu de null pour le JSON
	if posts == nil {
		posts = []domain.PostRequest{}
	}

	c.JSON(http.StatusOK, posts)
}
