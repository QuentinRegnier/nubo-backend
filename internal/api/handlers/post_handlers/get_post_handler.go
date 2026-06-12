package post_handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
	"github.com/gin-gonic/gin"
)

// GetPostHandler godoc
// @Summary      Récupérer un ou plusieurs posts
// @Description  Récupère une liste de posts en masse depuis leurs IDs (Forage en cascade L1 -> L2 -> L3).
// @Description  Le système filtre automatiquement les contenus selon la matrice de visibilité stricte (Public, Abonnés, Amis, Soft Delete).
// @Description  Cette route nécessite une authentification par JWT et une signature HMAC valide.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  ✅ **200 OK (Succès partiel ou total) :**
// @Description  * Retourne toujours un tableau. Si un post est inaccessible (privé, supprimé, banni), l'erreur est intégrée dans l'objet de réponse du post spécifique pour ne pas bloquer le reste de la liste.
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * Le paramètre 'ids' est manquant dans l'URL.
// @Description  * Limite dépassée : impossible de demander plus de 50 posts simultanément (Bouclier statique).
// @Description  * Aucun ID valide n'a pu être extrait.
// @Description
// @Description  🟠 **401 Unauthorized (Authentification) :**
// @Description  * Token JWT invalide, expiré ou utilisateur non identifié.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        ids           query  string true "Liste d'IDs séparés par des virgules (ex: ?ids=123,456)"
// @Success      200  {array}   post_models.GetPostOutput "Liste des posts hydratés (avec médias et commentaires) et/ou erreurs d'accès unitaires"
// @Failure      400  {object}  domain.ErrorResponse "Paramètre manquant ou limite de 50 IDs dépassée"
// @Failure      401  {object}  domain.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Router       /post [get]
func GetPostHandler(c *gin.Context) {
	// 1. Authentification
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		fmt.Printf("❌ Erreur authentification : %v\n", err)
		c.JSON(http.StatusUnauthorized, gin.H{"nubo_error": "Utilisateur non identifié"})
		return
	}

	// 2. Récupération des données (Parsing du query param 'ids')
	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Le paramètre 'ids' est requis"})
		return
	}

	// Extraction et conversion de la liste des IDs
	strIDs := strings.Split(idsParam, ",")
	if len(strIDs) > 50 {
		// Bouclier statique : on empêche de demander 10 000 posts d'un coup
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Limite maximum fixée à 50 IDs par requête"})
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
		c.JSON(http.StatusBadRequest, gin.H{"nubo_error": "Aucun ID valide fourni"})
		return
	}

	// Nettoyage des doublons potentiels envoyés par le client
	postIDs = pkg.SliceUniqueInt64(postIDs)

	input := post_models.GetPostInput{
		UserID:  userID,
		PostIDs: postIDs,
	}

	// 3. 4. 5. & 6. Envoi dans le service, vérification des droits et empaquetage
	results := post_service.GetPosts(c.Request.Context(), input)

	// 7. Renvoi des données
	c.JSON(http.StatusOK, results)
}
