package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// LogoutHandler godoc
// @Summary      Déconnecter l'appareil courant
// @Description  Révoque la session de l'appareil courant de l'utilisateur (identifié via le JWT). L'opération est idempotente.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée. Requiert un JWT valide et une signature HMAC.
// @Description  - Required permissions or roles: Aucune.
// @Description  - Relevant middleware: JWTMiddleware (authentification), HMACMiddleware (intégrité), MaxBodySize, RateLimiter, CORS, Recovery.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: Aucun corps requis. Les identifiants (`userID` et `firebase_installation_id`) sont extraits du contexte injecté par le middleware JWT.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Extraction du `userID` et du `firebase_installation_id` depuis le contexte de la requête.
// @Description  2. Si aucun ID d'installation n'est présent (ex: mode dev mal configuré), l'opération est ignorée (succès silencieux).
// @Description  3. Recherche de la session existante en cascade (L1 RAM -> L2 MongoDB -> L3 PostgreSQL).
// @Description  4. Si la session est introuvable à toutes les couches, le processus s'arrête (Idempotence : déjà déconnecté).
// @Description  5. Suppression immédiate de la session et de ses index dans le cache RAM (L1).
// @Description  6. Mise en file d'attente (Write-Behind vers Redis) de l'ordre de suppression physique (Hard Delete) pour que les workers nettoient MongoDB et PostgreSQL.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: JSON `{"message": "Déconnexion réussie"}`.
// @Description  - Persistence guarantees: Suppression asynchrone (Write-Behind). La session est immédiatement inutilisable en mémoire.
// @Description  - Side effects: Nettoyage du cache applicatif (L1) et de ses index.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  *(Aucune erreur publique n'est retournée explicitement par le handler lui-même en dehors des erreurs génériques de middleware (ex: 401 Unauthorized si le JWT est invalide). Le processus est totalement idempotent.)*
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Success      200  {object}  map[string]string "Message de succès"
// @Router       /logout [post]
func LogoutHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	firebaseInstallationIDs := c.GetString("firebase_installation_id")
	if firebaseInstallationIDs == "" {
		firebaseInstallationIDs = c.GetString("dev")
	}

	if err := auth_service.Logout(c.Request.Context(), callerID, firebaseInstallationIDs); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Déconnexion réussie"})
}
