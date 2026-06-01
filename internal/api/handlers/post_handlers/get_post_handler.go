package post_handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"
)

// GetPostHandler godoc
// @Summary      Récupérer un ou plusieurs posts
// @Description  Récupère une liste de posts depuis leurs IDs. Gère la visibilité (Public, Abonnés, Soft Delete).
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        ids           query  string true "Liste d'IDs séparés par des virgules (ex: ?ids=123,456)"
// @Success      200  {array}   post_service.PostFetchResult
// @Failure      400  {object}  domain.ErrorResponse
// @Failure      401  {object}  domain.ErrorResponse
// @Router       /post [get]
func GetPostHandler(c *gin.Context) {
	// 1. Authentification
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilisateur non identifié"})
		return
	}

	// 2. Récupération des données (Parsing du query param 'ids')
	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Le paramètre 'ids' est requis"})
		return
	}

	// Extraction et conversion de la liste des IDs
	strIDs := strings.Split(idsParam, ",")
	if len(strIDs) > 50 {
		// Bouclier statique : on empêche de demander 10 000 posts d'un coup
		c.JSON(http.StatusBadRequest, gin.H{"error": "Limite maximum fixée à 50 IDs par requête"})
		return
	}

	var postIDs []int64
	for _, strID := range strIDs {
		id, errParse := strconv.ParseInt(strings.TrimSpace(strID), 10, 64)
		if errParse == nil && id > 0 {
			postIDs = append(postIDs, id)
		}
	}

	if len(postIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aucun ID valide fourni"})
		return
	}

	// Nettoyage des doublons potentiels envoyés par le client
	postIDs = pkg.SliceUniqueInt64(postIDs)

	// 3. 4. 5. & 6. Envoi dans le service, vérification des droits et empaquetage
	results := post_service.GetPosts(c.Request.Context(), userID, postIDs)

	// 7. Renvoi des données
	c.JSON(http.StatusOK, results)
}
