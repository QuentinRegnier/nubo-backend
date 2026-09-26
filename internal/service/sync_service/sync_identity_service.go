package sync_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// ############################################################################
// # SERVICE : SYNCHRONISATION DE L'IDENTITÉ (COLD START APP)
// ############################################################################

// SyncIdentity synchronise uniquement l'identité (Profil et Settings) pour le Cold Start de l'App.
// Utilise le principe de Delta Sync : on ne renvoie la donnée que si elle a muté côté serveur.
func SyncIdentity(ctx context.Context, input sync_models.SyncIdentityInput) (sync_models.SyncIdentityOutput, error) {
	output := sync_models.SyncIdentityOutput{
		ProfileUpdated:  false,
		SettingsUpdated: false,
	}

	// ── ÉTAPE 1 : DELTA SYNC DU PROFIL UTILISATEUR (CASCADE L2 -> L3) ───────
	// Note architecturale : Pas de L1 ici car le Speed Cache ne stocke pas les données privées (Email, Phone).

	userPayload, errMongo := mongo.MongoLoadUser(input.UserID, "", "", "")
	if errMongo != nil && errMongo.Error() != "mongo: no documents in result" {
		logger.Log.Warn().Err(errMongo).Msg("Avertissement L2 Mongo lors du SyncIdentity")
	}

	if userPayload.ID == 0 {
		// FALLBACK L3 (PostgreSQL)
		var errPg error
		userPayload, errPg = postgres.FuncLoadUser(input.UserID, "", "", "")
		if errPg != nil {
			return output, nubo_error.NewInternal() // 500 générique au client, log l'erreur SQL
		}

		if userPayload.ID == 0 {
			// L'utilisateur n'existe vraiment pas dans le système.
			return output, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Profil introuvable.", nil)
		}

		// AUTO-GUÉRISON L3 -> L2 (Asynchrone via Worker)
		// L'hydratation L1 du SpeedCache se fera automatiquement si l'utilisateur met à jour son profil plus tard.
		go func(u auth_models.UserPayload) {
			bgCtx := context.Background()
			_ = redis.EnqueueDB(bgCtx, u.ID, 0, redis.EntityUser, redis.ActionUpdate, u, redis.TargetMongo)
		}(userPayload)
	}

	// Traitement du Profil si trouvé et mis à jour récemment
	if userPayload.UpdatedAt > input.ProfileUpdatedAt {
		output.ProfileUpdated = true

		// MAPPING SÉCURISÉ : On transfère uniquement les champs publics et autorisés au client
		output.Profile = auth_models.UserProfileView{
			ID:        userPayload.ID,
			Username:  userPayload.Username,
			Email:     userPayload.Email,
			Phone:     userPayload.Phone,
			FirstName: userPayload.FirstName,
			LastName:  userPayload.LastName,
			Birthdate: userPayload.Birthdate,
			Sex:       userPayload.Sex,
			Bio:       userPayload.Bio,
			Grade:     userPayload.Grade,
			Location:  userPayload.Location,
			School:    userPayload.School,
			Work:      userPayload.Work,
			Badges:    userPayload.Badges,
			CreatedAt: userPayload.CreatedAt,
			UpdatedAt: userPayload.UpdatedAt,
			IsOnline:  cache_service.IsUserOnline(ctx, userPayload.ID), // O(1) L1 Call
		}

		// HYDRATATION DE L'AVATAR (Composition par Valeur avec HMAC de sécurité)
		if userPayload.ProfilePictureID > 0 {
			if mediaView, errMedia := media_service.GenerateMediaViewCascade(ctx, userPayload.ProfilePictureID, userPayload.ID, 0, input.UserID); errMedia == nil {
				output.Avatar = mediaView
			}
		}
	}

	// ── ÉTAPE 2 : DELTA SYNC DES PARAMÈTRES (CASCADE COMPLÈTE EXISTANTE) ────

	settingsPayload, errSettings := object_cache_service.GetUserSettingsCascade(ctx, input.UserID)
	if errSettings != nil {
		// On loggue, mais on ne fait pas crasher l'identité entière pour un échec de settings.
		logger.Log.Warn().Err(errSettings).Msg("Impossible de récupérer les UserSettings lors du SyncIdentity")
	} else if settingsPayload.ID != 0 {
		if settingsPayload.UpdatedAt > input.SettingsUpdatedAt {
			output.SettingsUpdated = true
			output.Settings = settingsPayload
		}
	}

	return output, nil
}
