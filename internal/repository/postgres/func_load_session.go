package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

func FuncLoadSession(ID int64, UserId int64, DeviceToken string, MasterToken string) (models.SessionsRequest, error) {
	fmt.Println("FuncLoadSession called with:", ID, UserId, DeviceToken, MasterToken)
	const functionID = 3

	// 1. Vérification que les champs sont non nuls
	if ID == -1 && UserId == -1 && DeviceToken == "" && MasterToken == "" {
		return models.SessionsRequest{}, fmt.Errorf("erreur: champs requis manquants pour FuncLoadSession (ID %d)", functionID)
	}

	// 2. Préparation des arguments (gestion des types spéciaux)
	args := make([]any, 4)
	args[0] = ID          // p_session_id (ex: UUID)
	args[1] = UserId      // p_user_id (ex: UUID)
	args[2] = DeviceToken // p_device_token (ex: "token_string")
	args[3] = MasterToken // p_master_token (ex: "master_token_string")

	if ID == -1 {
		args[0] = nil
	}
	if UserId == -1 {
		args[1] = nil
	}
	if DeviceToken == "" {
		args[2] = nil
	}
	if MasterToken == "" {
		args[3] = nil
	}

	// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
	sqlStatement := `
		SELECT * FROM auth.func_load_sessions($1, $2, $3, $4)
	`

	// 4. Exécution via la connexion partagée du package 'db'
	//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
	var res models.SessionsRequest
	var deviceInfoBytes []byte
	var deviceToken sql.NullString // Le token peut être NULL en base (stocké en JSON string ou NULL)

	// not used variable
	res.CurrentSecret = ""
	res.LastSecret = ""
	res.LastJWT = ""
	res.ToleranceTime = time.Time{}

	err := postgres.PostgresDB.QueryRow(sqlStatement, args...).Scan(
		&res.ID,
		&res.UserID,
		&res.MasterToken,
		&deviceToken, // <-- Scan sécurisé
		&deviceInfoBytes,
		pq.Array(&res.IPHistory), // <-- pq.Array obligatoire
		&res.CreatedAt,
		&res.ExpiresAt,
	)

	if err != nil {
		// --- CORRECTION : Gérer le cas où aucune session n'est trouvée ---
		if errors.Is(err, sql.ErrNoRows) {
			// Ce n'est pas une erreur technique, juste qu'il n'y a pas de session.
			// On renvoie une structure vide et "pas d'erreur".
			return models.SessionsRequest{}, nil
		}
		// ---------------------------------------------------------------

		return models.SessionsRequest{}, fmt.Errorf("erreur lors de l'exécution de FuncLoadSession (ID %d): %w", functionID, err)
	}

	// Traitement des données
	if deviceToken.Valid {
		res.DeviceToken = deviceToken.String
	}

	if len(deviceInfoBytes) > 0 {
		_ = json.Unmarshal(deviceInfoBytes, &res.DeviceInfo)
	}

	return res, nil
}
