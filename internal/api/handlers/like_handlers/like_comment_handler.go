package like_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/service/like_service"
	"github.com/gin-gonic/gin"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
)

// LikeCommentHandler godoc
// @Summary      Aimer ou désaimer un commentaire
// @Description  Ajoute ou retire un Like sur un commentaire spécifique. Ce endpoint est synchrone pour les compteurs mais asynchrone pour la persistance.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `comment_id` (int64), `action` (chaîne: 'like' ou 'unlike').
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation GIN** : Vérifie la structure JSON et limite la valeur de "action".
// @Description  2. **Idempotence RAM** : Applique un verrou ultra-rapide (Redis) pour empêcher le spam clic de surcharger les workers. Si l'action a déjà été exécutée, la requête est acceptée silencieusement.
// @Description  3. **Vérification d'Existence** : Le commentaire est recherché (Cascade L1/L2/L3) ; rejet s'il est supprimé ou introuvable.
// @Description  4. **Incrémentation L1** : Le compteur `LikeCount` est modifié instantanément dans le Cache L1 (Object Cache). Le ZSET des classements de commentaires (Tri à bulle) est ajusté.
// @Description  5. **Délégation Worker** : Un payload de Like est mis en file d'attente Redis Write-Behind pour être persisté plus tard en BDD.
// @Description  6. **Notification** : Une notification push en temps réel est déclenchée asynchrone pour l'auteur du commentaire (s'il n'est pas le requérant lui-même).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: JSON avec `{"message": "Action prise en compte"}`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request — Payload invalide**
// @Description  - **Trigger:** Le payload JSON est manquant, mal formaté ou l'attribut `action` ne vaut ni `like` ni `unlike`.
// @Description  - **Execution stage:** Phase de binding GIN et validation.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeInvalidPayload` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  🟠 **401 Unauthorized — Authentification échouée**
// @Description  - **Trigger:** Le token JWT est absent, invalide ou expiré.
// @Description  - **Execution stage:** Middleware d'authentification ou extraction du profil.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeUnauthorized` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  ⚫ **404 Not Found — Commentaire introuvable**
// @Description  - **Trigger:** Le Commentaire référencé par `comment_id` n'existe pas ou son statut est "Soft Delete".
// @Description  - **Execution stage:** Étape de chargement en cascade du Commentaire, après avoir passé le verrou d'idempotence.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeNotFound` (renvoyé dans le champ JSON `code`).
// @Description
// @Description  ⚫ **500 Internal Server Error — Erreur inattendue**
// @Description  - **Trigger:** Impossible d'interroger la base PostgreSQL lors du fallback L3 du chargement du commentaire.
// @Description  - **Execution stage:** Tentative de récupération L3 dans `getCommentCascade`.
// @Description  - **Response:** Structure JSON `numan_error.PublicErrorResponse`.
// @Description  - **Error code:** `numan_error.CodeInternalError` (renvoyé dans le champ JSON `code`).
// @Tags         likes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   like_models.LikeCommentInput true "Payload pour aimer ou désaimer un commentaire"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /like/comment/set [post]
func LikeCommentHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input like_models.LikeCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou action non reconnue ('like'/'unlike' attendu).", err))
		return
	}

	err = like_service.ToggleCommentLike(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action prise en compte"})
}
