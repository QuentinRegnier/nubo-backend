package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// SignUp godoc
// @Summary      Créer un compte utilisateur
// @Description  Inscription complète avec upload d'avatar et données JSON.
// @Description
// @Description  **Règles de validation & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request (Erreurs client) :**
// @Description  * `The 'data' field containing the JSON is required` : Tu as oublié d'envoyer le champ texte 'data'.
// @Description  * `Invalid JSON format in 'data': ...` : Ton JSON est mal écrit (virgule manquante, accolade, etc).
// @Description  * `Invalid date format. Expected format: ddmmaaaa` : La date de naissance n'est pas bonne.
// @Description  * `Gender must be 0, 1, 2, or null` : Tu as envoyé un entier invalide pour le sexe.
// @Description  * `Impossible to read image file` : Le fichier image est corrompu ou illisible.
// @Description
// @Description  🟠 **409 Conflict (Doublons) :**
// @Description  * `This username is already taken` : Le pseudo est déjà en base.
// @Description
// @Description  ⚫ **500 Internal Server Error (Problèmes serveur) :**
// @Description  * `Internal error (image upload)` : MinIO est down ou mal configuré.
// @Description  * `Internal error (token generation)` : Problème avec la signature JWT.
// @Description  * `database error` : Postgres ou Mongo ne répondent pas.
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        profile_picture formData file   false "Photo de profil (Image)"
// @Param        data            formData string true  "Données JSON (domain.SignUpInput)"
// @Success      200  {object}  domain.SignUpResponse
// @Failure      400  {object}  domain.ErrorResponse "Données invalides (Voir liste ci-dessus)"
// @Failure      409  {object}  domain.ErrorResponse "Conflit (Pseudo pris)"
// @Failure      500  {object}  domain.ErrorResponse "Erreur Serveur"
// @Router       /signup [post]
func SignUpHandler(c *gin.Context) {
	var input domain.SignUpInput
	// --- A. RÉCUPÉRATION DES DONNÉES MIXTES (Multipart) ---
	jsonData := c.PostForm("data")
	if jsonData == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
		return
	}
	if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
		return
	}
	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var req domain.UserRequest
	req.ID = -1
	req.Username = input.Username
	if service.IsUnique(mongo.Users, "username", req.Username) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This username is already taken"})
		return
	}
	req.Email = input.Email
	if service.IsUnique(mongo.Users, "email", req.Email) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This email is already taken"})
		return
	}
	req.EmailVerified = false // Par défaut
	req.Phone = input.Phone
	if service.IsUnique(mongo.Users, "phone", req.Phone) == 0 {
		c.JSON(http.StatusConflict, domain.ErrorResponse{Error: "This phone number is already taken"})
		return
	}
	req.PhoneVerified = false // Par défaut
	req.PasswordHash = input.PasswordHash
	req.FirstName = input.FirstName
	req.LastName = input.LastName
	parsedBirthdate, err := time.Parse("02012006", input.Birthdate)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid date format. Expected format: ddmmaaaa"})
		return
	}
	req.Birthdate = parsedBirthdate
	if input.Gender != nil {
		g := *input.Gender
		if g < 0 || g > 2 {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Gender must be 0, 1, 2, or null"})
			return
		}
		req.Sex = g
	} else {
		// Gérer le cas null si nécessaire, par défaut int vaut 0.
		// Si 0 est une valeur valide (ex: Homme), il faut définir une logique pour "Non spécifié".
	}
	req.Bio = pkg.CleanStr(input.Bio) // Nettoyage immédiat
	req.Grade = 0                     // Par défaut
	req.Location = input.Location
	req.School = input.School
	req.Work = input.Work
	req.Badges = []string{}
	req.Desactivated = false // Par défaut
	req.Banned = false       // Par défaut
	req.BanReason = ""
	req.BanExpiresAt = time.Time{}
	req.CreatedAt = time.Time{}
	req.UpdatedAt = time.Time{}

	// --- C. LOGIQUE UPLOAD ---
	fileHeader, errFile := c.FormFile("profile_picture")

	// --- D. CRÉATION USER & TOKEN ---
	var sessions domain.SessionsRequest
	sessions.ID = -1     // Auto-généré
	sessions.UserID = -1 // Sera défini après création user
	sessions.MasterToken = ""
	sessions.DeviceToken = input.DeviceToken
	sessions.DeviceInfo = input.DeviceInfo
	sessions.IPHistory = []string{c.ClientIP()}
	sessions.CurrentSecret = ""
	sessions.LastSecret = sessions.DeviceToken
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	sessions.CreatedAt = time.Time{}
	sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	// Persistance en base de données
	// Les arguments 'desactivated', 'banned', etc. sont maintenant DANS 'req'.
	// J'assume que la signature de FuncCreateUser a changé pour accepter (req, token, ...).

	userID, JWT, err := service.CreateUser(&req, &sessions, fileHeader, errFile)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, domain.SignUpResponse{
			UserID:           userID,
			MasterToken:      sessions.MasterToken,
			JWT:              JWT,
			ExpiresAt:        sessions.ExpiresAt,
			Message:          "User created successfully",
			ProfilePictureID: req.ProfilePictureID, // On renvoie l'UUID au front pour affichage direct
		})
	} else {
		fmt.Printf("❌ ERREUR CRITIQUE DATABASE (CreateUser): %v\n", err)
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error"})
	}
}

// Login godoc
// @Summary      Connecter un utilisateur
// @Description  Authentifie un utilisateur via email/password et renvoie son profil complet + token.
// @Description
// @Description  **Règles & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :**
// @Description  * `The 'data' field containing the JSON is required` : Champ 'data' manquant.
// @Description  * `Invalid JSON format in 'data'` : Le JSON envoyé est mal formé.
// @Description
// @Description  🟠 **401 Unauthorized :**
// @Description  * `Invalid email or password` : Identifiants incorrects ou utilisateur introuvable.
// @Description
// @Description  ⛔ **403 Forbidden :**
// @Description  * `Account deactivated` : Le compte a été désactivé.
// @Description  * `Account banned` : Le compte a été banni.
// @Description
// @Description  ⚫ **500 Internal Server Error :**
// @Description  * `database error` : Erreur technique interne.
// @Tags         users
// @Accept       json,multipart/form-data
// @Produce      json
// @Param        data    formData string            false "Données JSON (si multipart/form-data)"
// @Param        request body     domain.LoginInput false "Données JSON (si application/json)"
// @Success      200  {object}  domain.LoginResponse
// @Failure      400  {object}  domain.ErrorResponse "Invalid request format"
// @Failure      401  {object}  domain.ErrorResponse "Identifiants invalides"
// @Failure      403  {object}  domain.ErrorResponse "Compte bloqué/banni"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur"
// @Failure      400  {object}  domain.ErrorResponse "Adresse IP invalide"
// @Router       /login [post]
func LoginHandler(c *gin.Context) {
	var input domain.LoginInput
	var user domain.UserRequest
	var sessions domain.SessionsRequest
	var err error

	// --- A. RÉCUPÉRATION DES DONNÉES ---
	// 1. Récupération via form-data (exactement comme SignUp)
	jsonData := c.PostForm("data")

	// Si on veut être souple et accepter aussi le raw JSON (optionnel mais pratique)
	if jsonData == "" && c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format: " + err.Error()})
			return
		}
	} else {
		// Logique Form-Data (Votre demande)
		if jsonData == "" {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
			return
		}
		if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
			return
		}
	}

	IPAddress := []string{c.ClientIP()}

	// --- B. MAPPING VERS STRUCTURE INTERNE ---
	var JWT string
	user, sessions, JWT, err = service.Login(input, IPAddress)

	if err == nil {
		//go StartWebsocket()

		c.JSON(http.StatusOK, domain.LoginResponse{
			UserID:        user.ID,
			Username:      user.Username,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			Phone:         user.Phone,
			PhoneVerified: user.PhoneVerified,
			FirstName:     user.FirstName,
			LastName:      user.LastName,
			Birthdate:     user.Birthdate,
			Sex:           user.Sex,
			Bio:           user.Bio,
			Grade:         user.Grade,
			Location:      user.Location,
			School:        user.School,
			Work:          user.Work,
			Badges:        user.Badges,
			Desactivated:  user.Desactivated,
			Banned:        user.Banned,
			BanReason:     user.BanReason,
			BanExpiresAt:  user.BanExpiresAt,
			CreatedAt:     user.CreatedAt,
			UpdatedAt:     user.UpdatedAt,
			MasterToken:   sessions.MasterToken,
			JWT:           JWT,
			ExpiresAt:     sessions.ExpiresAt,
			Message:       "Login successful",
		})
	} else {
		// 🔍 DIAGNOSTIC PRÉCIS
		// Si le service renvoie une erreur "Identifiants invalides" ou "Introuvable"
		if err == domain.ErrInvalidCredentials || err == domain.ErrNotFound {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Invalid email or password"})
			return
		}
		if err == domain.ErrDesactivated {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{Error: "Account deactivated"})
			return
		}
		if err == domain.ErrBanned {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{Error: "Account banned"})
			return
		}

		// Sinon, c'est une vraie erreur technique (ex: Redis down)
		fmt.Printf("🚨 VRAIE ERREUR INTERNE : %v\n", err)
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "database error: " + err.Error()})
		return
	}
}

// RenewJWT godoc
// @Summary      Renouveler le JWT (Ratchet Rotation)
// @Description  Génère un nouveau JWT pour l'utilisateur et effectue une rotation de sécurité des secrets (Ratchet).
// @Description  Cette route est critique et nécessite une signature HMAC valide basée sur le secret actuel de la session.
// @Description
// @Description  **Mécanisme :**
// @Description  1. Vérifie la signature HMAC du body avec les headers de sécurité.
// @Description  2. Identifie la session via l'ID utilisateur et le `X-Secret`.
// @Description  3. Calcule le prochain secret (N+1) et met à jour l'historique (Ratchet).
// @Description  4. Renvoie le nouveau JWT.
// @Description
// @Description  **Règles & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :**
// @Description  * `Erreur lecture body` : Impossible de lire le corps de la requête.
// @Description  * `Invalid JSON format` : Le JSON envoyé est mal formé.
// @Description  * `Headers de sécurité manquants` : Il manque `Authorization`, `X-Secret`, `X-Signature` ou `X-Timestamp`.
// @Description
// @Description  🟠 **401 Unauthorized :**
// @Description  * `Signature HMAC invalide` : La signature ne correspond pas au contenu (tentative de falsification).
// @Description  * `Session invalide ou Secret incorrect` : Le secret fourni ne correspond à aucune session active pour cet utilisateur (ou désynchronisation Ratchet).
// @Description
// @Description  ⚫ **500 Internal Server Error :**
// @Description  * `Erreur génération token` : Échec de la création du JWT.
// @Description  * `Erreur rotation secrets` : Impossible de mettre à jour Redis (Ratchet bloqué).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <Last_JWT>"
// @Param        X-Secret      header string true "Secret actuel de la session"
// @Param        X-Signature   header string true "Signature HMAC calculée"
// @Param        X-Timestamp   header string true "Timestamp de la requête"
// @Success      200  {object}  domain.RenewJWTResponse
// @Failure      400  {object}  domain.ErrorResponse "Requête invalide"
// @Failure      401  {object}  domain.ErrorResponse "Authentification / Signature refusée"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur critique"
// @Router       /renew-jwt [post]
func RenewJWT(c *gin.Context) {
	// 1. Lire le Body uniquement pour la signature HMAC (Raw bytes)
	// On ne fait plus de json.Unmarshal ici car on récupère les infos du Token
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Erreur lecture body"})
		return
	}
	// On n'a pas besoin de restaurer le body avec NopCloser car on ne le relit plus après

	// 2. Récupération des Headers
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	}
	clientSecret := c.GetHeader("X-Secret")
	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if authHeader == "" || clientSecret == "" || clientHMAC == "" || clientTs == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Headers de sécurité manquants"})
		return
	}

	// 3. EXTRACTION DES DONNÉES DU JWT (Même périmé)
	// On utilise ParseUnverified de la lib jwt/v5
	token, _, err := new(jwt.Parser).ParseUnverified(authHeader, jwt.MapClaims{})
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Token illisible"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Claims JWT invalides"})
		return
	}

	// A. Récupération UserID ("sub")
	sub, err := claims.GetSubject()
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "UserID manquant dans le token"})
		return
	}
	userID, err := strconv.ParseInt(sub, 10, 64) // Conversion en int64
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Format UserID invalide"})
		return
	}

	// B. Récupération DeviceToken ("dev")
	deviceToken, ok := claims["dev"].(string)
	if !ok || deviceToken == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "DeviceToken manquant dans le token"})
		return
	}

	// 4. Vérification HMAC
	// On signe toujours avec le bodyBytes (même vide) pour garantir l'intégrité de la requête
	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)

	if !security.CheckHMAC(stringToSign, clientSecret, clientHMAC) {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Signature HMAC invalide"})
		return
	}

	// 5. Génération Nouveau JWT (Action Serveur)
	// IMPORTANT : On remet le deviceToken dans le nouveau JWT pour la suite !
	newJWT, err := pkg.GenerateToken(userID, deviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur génération token"})
		return
	}

	// 6. Rotation du Ratchet & Mise à jour Session
	// On utilise le userID extrait du token et le authHeader comme "LastJWT"
	if err := security.RotateRatchet(c, userID, clientSecret, authHeader); err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Session invalide ou Secret incorrect"})
		return
	}

	// 7. PRÉPARATION DE LA RÉPONSE SIGNÉE
	// On prépare l'objet réponse
	respData := domain.RenewJWTResponse{
		Token:   newJWT,
		Message: "Renouvellement OK",
	}

	// A. Sérialisation manuelle en JSON pour la signature
	respBytes, err := json.Marshal(respData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur encoding réponse"})
		return
	}

	// B. Génération du Timestamp et Signature
	respTs := fmt.Sprintf("%d", time.Now().Unix())

	// C. Build StringToSign (Response Binding)
	// On signe : METHOD | PATH | TS_REPONSE | BODY_REPONSE
	stringToSignResp := security.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		respTs,
		string(respBytes),
	)

	// D. Calcul HMAC
	// Règle : On utilise le secret qui a validé la requête (clientSecret)
	// C'est ce secret qui est devenu 'LastSecret' dans la BDD après la rotation.
	h := hmac.New(sha256.New, []byte(clientSecret))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	// E. Ajout des Headers
	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)

	// F. Envoi de la réponse
	c.Data(http.StatusOK, "application/json", respBytes)
}

// RefreshMaster godoc
// @Summary      Hard Refresh (Master Token Rotation)
// @Description  Réinitialise toute la chaîne de sécurité (Ratchet, JWT, Secrets) en générant un nouveau MasterToken.
// @Description  Cette route est l'ultime recours ("Last Resort") lorsque le Ratchet est désynchronisé ou que le JWT est expiré depuis trop longtemps.
// @Description
// @Description  **Mécanisme de Résilience :**
// @Description  1. Recherche la session via le MasterToken dans **Redis**.
// @Description  2. Si introuvable (crash cache), cherche dans **MongoDB**.
// @Description  3. Si introuvable, cherche dans **PostgreSQL** (Source de vérité).
// @Description  4. Si trouvé, valide la signature HMAC et réinitialise tout.
// @Description
// @Description  **Actions Serveur :**
// @Description  * Génération de `NewMasterToken` et `NewJWT`.
// @Description  * Reset du Ratchet (Secret 0 = NewMaster, Secret 1 = DeviceToken).
// @Description  * Mise à jour asynchrone de Postgres et Mongo pour persister le nouveau MasterToken.
// @Description
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string false "Bearer <Current_JWT> (Optionnel, pour continuité)"
// @Param        X-Signature   header string true  "HMAC calculé avec l'ANCIEN MasterToken"
// @Param        X-Timestamp   header string true  "Timestamp de la requête"
// @Param        request       body     domain.RefreshMasterInput true "Données de reset (MasterToken, UserID, Username)"
// @Success      200  {object}  domain.RefreshMasterResponse "Nouveaux identifiants générés"
// @Failure      400  {object}  domain.ErrorResponse "Format invalide ou Headers manquants"
// @Failure      401  {object}  domain.ErrorResponse "MasterToken introuvable ou Signature HMAC invalide"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur critique (Génération/Sauvegarde)"
// @Router       /auth/refresh-master [post]
func RefreshMaster(c *gin.Context) {
	// 1. Lecture du Body (Nécessaire pour le calcul HMAC manuel plus bas)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Erreur lecture body"})
		return
	}

	// [IMPORTANT] Restaurer le body pour que c.PostForm puisse le lire ensuite
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 2. Parsing des données (Support Form-Data ET Raw JSON)
	var input domain.RefreshMasterInput
	jsonData := c.PostForm("data")

	if jsonData != "" {
		// CAS 1 : Multipart/Form-Data (celui que tu veux utiliser)
		if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
			return
		}
	} else {
		// CAS 2 : Raw JSON (Fallback, au cas où)
		// Si 'data' est vide, on essaie de parser le body entier
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &input); err != nil {
				c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format"})
				return
			}
		} else {
			// Aucun contenu trouvé
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
			return
		}
	}

	// 2. Headers
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	} else {
		authHeader = "" // Cas NULL accepté (perte du JWT)
	}

	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if clientHMAC == "" || clientTs == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Headers de sécurité manquants"})
		return
	}

	// 5. Récupération de la Session (Cascade : Redis -> Mongo -> Postgres)
	var sessionRaw domain.SessionsRequest
	var sessionFound bool = false

	// A. Essai Redis
	sessionRaw, err = redis.RedisLoadSession(input.UserID, "", input.MasterToken, "")
	if err == nil && sessionRaw.ID != 0 && sessionRaw.MasterToken == input.MasterToken {
		sessionFound = true
	}

	// B. Essai Mongo (Si pas trouvé dans Redis)
	if !sessionFound {
		// Supposons que tu aies un repo mongo générique similaire
		sessionRaw, errMongo := mongo.MongoLoadSession(input.UserID, "", input.MasterToken, "")
		if errMongo == nil && sessionRaw.ID != 0 && sessionRaw.MasterToken == input.MasterToken {
			sessionFound = true
			// Repopulation Cache
			if errAdd := redis.RedisCreateSession(sessionRaw); errAdd != nil {
				fmt.Printf("⚠️ Warning: Echec repopulation Redis depuis Mongo: %v\n", errAdd)
			}
		}
	}

	// C. Essai Postgres (Si pas trouvé dans Mongo)
	if !sessionFound {
		sessionRaw, errPg := postgres.FuncLoadSession(-1, input.UserID, "", input.MasterToken)
		if errPg == nil && sessionRaw.ID != 0 && sessionRaw.MasterToken == input.MasterToken {
			sessionFound = true

			// 🚨 REPOPULATION MONGO (Backup via Queue)
			// On envoie une action CREATE vers Mongo car elle n'existait pas
			redis.EnqueueDB(c, sessionRaw.ID, 0, redis.EntitySession, redis.ActionCreate, sessionRaw, redis.TargetMongo)

			// 🚨 REPOPULATION REDIS (Cache Actif - Synchrone requis ici)
			if errAdd := redis.RedisCreateSession(sessionRaw); errAdd != nil {
				fmt.Printf("⚠️ Warning: Echec repopulation Redis depuis Postgres: %v\n", errAdd)
			}
		}
	}

	if !sessionFound {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "MasterToken invalide ou session introuvable (All sources failed)"})
		return
	}

	// 4. Vérification HMAC (Master Check)
	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	fmt.Printf("ContentToSign for Master Check: %s\n", contentToSign)                                                     // --- IGNORE ---
	fmt.Printf("Arguments for Master Check: Method=%s, Path=%s, Ts=%s\n", c.Request.Method, c.Request.URL.Path, clientTs) // --- IGNORE ---
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)
	fmt.Printf("StringToSign for Master Check: %s\n", stringToSign) // --- IGNORE ---
	if !security.CheckHMAC(stringToSign, input.MasterToken, clientHMAC) {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Signature HMAC invalide (Master Check)"})
		return
	}

	// 6. Génération des Nouveaux Credentials
	newMasterToken, err := pkg.GenerateToken(input.UserID, sessionRaw.DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur génération MasterToken"})
		return
	}

	newJWT, err := pkg.GenerateToken(input.UserID, sessionRaw.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur génération JWT"})
		return
	}

	// 7. Reset du Ratchet dans Redis
	if sessionRaw.CurrentSecret, err = security.ResetRatchet(c, sessionRaw.ID, newMasterToken, sessionRaw.DeviceToken, authHeader); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur reset Ratchet"})
		return
	}

	// 8. Mise à jour des Bases de Données (Redis Sync + Queue Async)

	// Mise à jour de l'objet local
	sessionRaw.MasterToken = newMasterToken
	sessionRaw.LastSecret = sessionRaw.DeviceToken
	sessionRaw.LastJWT = authHeader
	sessionRaw.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	sessionRaw.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	// A. Redis (Immédiat pour le cache)
	if errAdd := redis.RedisUpdateSession(sessionRaw); errAdd != nil {
		fmt.Printf("⚠️ Warning: Echec update Redis: %v\n", errAdd)
	}

	// TargetBoth : On veut mettre à jour Mongo (Doc) ET Postgres (Relationnel)
	// car le MasterToken a changé (info critique).
	redis.EnqueueDB(c, sessionRaw.ID, 0, redis.EntitySession, redis.ActionUpdate, sessionRaw, redis.TargetAll)

	// 9. PRÉPARATION DE LA RÉPONSE SIGNÉE
	respData := domain.RefreshMasterResponse{
		MasterToken: newMasterToken,
		Token:       newJWT,
		Message:     "Master Reset Successful",
	}

	// A. Sérialisation
	respBytes, err := json.Marshal(respData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur encoding réponse"})
		return
	}

	// B. Timestamp
	respTs := fmt.Sprintf("%d", time.Now().Unix())

	// C. StringToSign
	stringToSignResp := security.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		respTs,
		string(respBytes),
	)

	// D. Calcul HMAC
	// Règle : On utilise l'ANCIEN MasterToken (input.MasterToken) car le client ne connait pas encore le nouveau.
	h := hmac.New(sha256.New, []byte(input.MasterToken))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	// E. Headers
	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)

	// F. Envoi
	c.Data(http.StatusOK, "application/json", respBytes)
}
