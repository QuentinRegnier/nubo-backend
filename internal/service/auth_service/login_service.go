package auth_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : CONNEXION UTILISATEUR (LOGIN)
// ############################################################################

// Login retourne uniquement les tokens et l'ID utilisateur.
// Les métadonnées complètes (profil, avatar) seront appelées plus tard via /sync.
func Login(input auth_models.LoginInput, ipAddresses []string) (int64, auth_models.SessionsPayload, string, error) {
	logger.Log.Info().Str("email", input.Email).Msg("Tentative de connexion entrante...")

	var userPayload auth_models.UserPayload
	var sessionPayload auth_models.SessionsPayload
	ctx := context.Background()

	// ── ÉTAPE 1 : CHARGEMENT DE L'UTILISATEUR (CASCADE L2 -> L3) ────────────

	// Tentative L2 (MongoDB - Warm Storage)
	userPayload, errMongo := mongo.MongoLoadUser(-1, "", input.Email, "")
	if errMongo != nil || userPayload.ID == 0 {
		if errMongo != nil {
			logger.Log.Warn().Err(errMongo).Str("email", input.Email).Msg("Mongo L2 : Utilisateur absent ou erreur de connexion.")
		}

		// FALLBACK L3 (PostgreSQL - Cold Storage)
		var errPg error
		userPayload, errPg = postgresgo.FuncLoadUser(-1, "", input.Email, "")
		if errPg != nil {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal()
		}

		if userPayload.ID == 0 {
			// SÉCURITÉ : On ne dit jamais si l'email existe ou pas (Prévention de l'énumération de comptes)
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewUnauthorized(nubo_error.CodeUnauthorized, "L'email ou le mot de passe est incorrect.", nil)
		}

		// AUTO-GUÉRISON L3 -> L2 (Asynchrone via Queue)
		if errQueue := redis.EnqueueDB(ctx, userPayload.ID, 0, redis.EntityUser, redis.ActionCreate, &userPayload, redis.TargetMongo); errQueue != nil {
			logger.Log.Warn().Err(errQueue).Int64("user_id", userPayload.ID).Msg("Échec de la guérison L2 pour l'utilisateur")
		}
	}

	// ── ÉTAPE 2 : CONTRÔLE DE SÉCURITÉ ET STATUT DU COMPTE ──────────────────

	// Vérification du mot de passe
	if strings.TrimSpace(userPayload.PasswordHash) != strings.TrimSpace(input.PasswordHash) {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewUnauthorized(nubo_error.CodeUnauthorized, "L'email ou le mot de passe est incorrect.", nil)
	}

	// Vérification des suspensions
	if userPayload.Desactivated || userPayload.Banned {
		if userPayload.Desactivated {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewForbidden(nubo_error.CodeForbidden, "Ce compte est actuellement désactivé.", nil)
		}
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewForbidden(nubo_error.CodeForbidden, "Accès refusé : Ce compte a été banni.", nil)
	}

	// ── ÉTAPE 3 : GESTION DE LA SESSION DE L'APPAREIL (CASCADE L1->L2->L3) ──

	currentTime := time.Now().UTC()
	isNewSession := false
	deviceFirebaseID := input.FirebaseInstallationID

	// TENTATIVE L1 (RAM)
	sessionPayload, _ = cache_service.LoadSessionFromCache(ctx, userPayload.ID, deviceFirebaseID, "")

	if sessionPayload.ID == 0 {
		// FALLBACK L2 (Mongo)
		sessionPayload, _ = mongo.MongoLoadSession(userPayload.ID, deviceFirebaseID, "", "")

		if sessionPayload.ID == 0 {
			// FALLBACK L3 (Postgres)
			sessionPayload, _ = postgresgo.FuncLoadSession(-1, userPayload.ID, deviceFirebaseID, "")

			if sessionPayload.ID != 0 {
				// AUTO-GUÉRISON L3 -> L2
				go func(s auth_models.SessionsPayload) {
					bgCtx := context.Background()
					_ = redis.EnqueueDB(bgCtx, s.ID, s.UserID, redis.EntitySession, redis.ActionUpdate, s, redis.TargetMongo)
				}(sessionPayload)
			}
		}

		if sessionPayload.ID != 0 {
			// AUTO-GUÉRISON L3/L2 -> L1
			_ = cache_service.SetSessionInCache(ctx, sessionPayload)
		}
	}

	// HYDRATATION DE LA SESSION (Mise à jour ou Création)
	if sessionPayload.ID != 0 {
		// La session existait déjà, on met à jour les infos matérielles
		sessionPayload.DeviceInfo = input.DeviceInfo
		if len(ipAddresses) > 0 && !pkg.Exists(sessionPayload.IPHistory, ipAddresses[0]) {
			sessionPayload.IPHistory = append(sessionPayload.IPHistory, ipAddresses[0])
		}
	} else {
		// L'appareil est inconnu, on forge une nouvelle session
		isNewSession = true
		sessionPayload.ID = pkg.GenerateID()
		sessionPayload.UserID = userPayload.ID
		sessionPayload.CreatedAt = domain.TimeToMillis(currentTime)
		sessionPayload.FirebaseInstallationID = deviceFirebaseID
		sessionPayload.DeviceInfo = input.DeviceInfo

		if len(ipAddresses) > 0 {
			sessionPayload.IPHistory = []string{ipAddresses[0]}
		} else {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "L'adresse IP est requise pour authentifier une nouvelle connexion.", nil)
		}
	}

	// GESTION DES TOKENS SÉCURISÉS (Master & JWT)
	var errToken error
	sessionPayload.ExpiresAt = domain.TimeToMillis(currentTime.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second))

	sessionPayload.MasterToken, errToken = pkg.GenerateToken(userPayload.ID, deviceFirebaseID, variables.MasterTokenExpirationSeconds)
	if errToken != nil {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal()
	}

	sessionPayload.CurrentSecret = security.DeriveNextSecret(sessionPayload.FirebaseInstallationID, sessionPayload.MasterToken, sessionPayload.MasterToken, sessionPayload.FirebaseInstallationID)
	sessionPayload.LastSecret = sessionPayload.FirebaseInstallationID
	sessionPayload.LastJWT = ""
	sessionPayload.ToleranceTime = domain.TimeToMillis(time.Time{})

	newJWT, errJwt := pkg.GenerateToken(userPayload.ID, sessionPayload.FirebaseInstallationID, variables.JWTExpirationSeconds)
	if errJwt != nil {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal()
	}

	// ── ÉTAPE 4 : SYNCHRONISATION L1 & SPEED CACHE (Cold Start User) ────────

	if errSet := cache_service.SetSessionInCache(ctx, sessionPayload); errSet != nil {
		logger.Log.Warn().Err(errSet).Int64("session_id", sessionPayload.ID).Msg("Échec mise en cache L1 de la Session lors du Login")
	}

	// Vérification de la Timeline Utilisateur
	timelineKey := fmt.Sprintf("profile:posts:zset:%d", userPayload.ID)
	timelineExists, errTimeline := redis.Exists(ctx, timelineKey)
	if errTimeline != nil || !timelineExists {
		_ = cache_service.MarkUserTimelineEmpty(ctx, userPayload.ID)
	}

	// Vérification du Profil Lite (Pour affichage rapide UI)
	var liteUser lite_models.UserLiteRequest
	if errSpeed := redis.UsersLite.GetObject(ctx, userPayload.ID, &liteUser); errSpeed != nil {
		settingsPayload, _ := object_cache_service.GetUserSettingsCascade(ctx, userPayload.ID)
		_ = cache_service.AddUserToSpeedCache(ctx, userPayload, settingsPayload)
	}

	// ── ÉTAPE 5 : MISE EN FILE D'ATTENTE (Write-Behind) ─────────────────────

	dbAction := redis.ActionUpdate
	if isNewSession {
		dbAction = redis.ActionCreate
	}

	errQueue := redis.EnqueueDB(ctx, sessionPayload.ID, userPayload.ID, redis.EntitySession, dbAction, sessionPayload, redis.TargetAll)
	if errQueue != nil {
		// Loggué en Error car la persistance est brisée, mais on ne bloque pas le retour du token au client
		logger.Log.Error().Err(errQueue).Int64("session_id", sessionPayload.ID).Msg("Rupture du Write-Behind pour la session lors du Login")
	}

	return userPayload.ID, sessionPayload, newJWT, nil
}
