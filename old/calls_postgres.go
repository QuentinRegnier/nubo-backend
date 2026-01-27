package old

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// FuncCreateUser accepte maintenant directement le modèle RegisterRequest.
// profilePictureID est passé à part car req.ProfilePicture contient du Base64, pas l'UUID.
func FuncCreateUser(req domain.UserRequest, sessions domain.SessionsRequest) (int, time.Time, time.Time, int, time.Time, error) {

	const functionID = 1

	// --- 1. Préparation et Conversion des données ---

	// A. Conversion Date (*time.Time à partir de time.Time)
	var birthdatePtr *time.Time
	if !req.Birthdate.IsZero() {
		birthdatePtr = &req.Birthdate
	}

	// B. Conversion Phone (Struct -> String)
	// ADAPTEZ ICI : Si req.Phone a un champ Number, faites : phoneStr := req.Phone.Number
	phoneStr := fmt.Sprintf("%v", req.Phone)

	// C. Conversion Location (Struct -> String)
	// ADAPTEZ ICI : ex: locStr := req.Location.City + ", " + req.Location.Country
	locationStr := fmt.Sprintf("%v", req.Location)

	// --- 2. Remplissage des arguments SQL ---
	args := make([]any, 24)

	args[0] = req.Username
	args[1] = req.Email
	args[2] = phoneStr // La version string convertie
	args[3] = req.PasswordHash
	args[4] = req.FirstName
	args[5] = req.LastName
	args[6] = birthdatePtr // Le pointeur time.Time calculé
	args[7] = req.Sex      // Déjà *int dans la struct
	args[8] = req.Bio
	// Gestion UUID Profile Picture
	if req.ProfilePictureID <= 0 {
		args[9] = nil
	} else {
		args[9] = req.ProfilePictureID
	}
	args[10] = req.Grade
	args[11] = locationStr // La version string convertie
	args[12] = req.School
	args[13] = req.Work
	if len(req.Badges) == 0 {
		args[14] = nil
	} else {
		args[14] = pq.Array(req.Badges) // <--- Indispensable pour Postgres
	}
	args[15] = req.Desactivated
	args[16] = req.Banned
	if req.BanReason == "" {
		args[17] = nil
	} else {
		args[17] = req.BanReason
	}
	if req.BanExpiresAt.IsZero() {
		args[18] = nil
	} else {
		args[18] = req.BanExpiresAt
	}
	args[19] = sessions.MasterToken
	if sessions.DeviceInfo == nil {
		args[20] = nil
	} else {
		deviceInfoBytes, err := json.Marshal(sessions.DeviceInfo)
		if err != nil {
			// En cas d'erreur, on envoie un objet vide pour ne pas faire planter la requête
			args[20] = "{}"
		} else {
			args[20] = string(deviceInfoBytes)
		}
	}
	if sessions.DeviceToken == "" {
		args[21] = nil
	} else {
		// On utilise json.Marshal sur la string pour obtenir une string JSON valide
		// Ex: "mon_token" devient "\"mon_token\""
		tokenJson, err := json.Marshal(sessions.DeviceToken)
		if err != nil {
			// Fallback si échec (ne devrait pas arriver sur une string)
			args[21] = fmt.Sprintf(`"%s"`, sessions.DeviceToken)
		} else {
			args[21] = string(tokenJson) // <--- ICI : Conversion en format JSON
		}
	}
	if len(sessions.IPHistory) == 0 {
		args[22] = nil
	} else {
		args[22] = pq.Array(sessions.IPHistory)
	}
	args[23] = sessions.ExpiresAt

	// --- 3. Exécution SQL ---
	sqlStatement := `
		SELECT * FROM auth.func_create_user($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`

	var returnedUUID int
	var createdAtUser time.Time
	var updatedAtUser time.Time
	var returnedSessionUUID int
	var createdAtSession time.Time

	// Assurez-vous que db.PostgresDB est accessible
	err := postgres.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID, &createdAtUser, &updatedAtUser, &returnedSessionUUID, &createdAtSession)
	if err != nil {
		return 0, time.Time{}, time.Time{}, 0, time.Time{}, fmt.Errorf("erreur FuncCreateUser (ID %d): %w", functionID, err)
	}

	return returnedUUID, createdAtUser, updatedAtUser, returnedSessionUUID, createdAtSession, err
}
func FuncCreateSession(UserID int64, MasterToken string, DeviceInfo any, DeviceToken string, IPHistory []string, ExpiresAt time.Time) (int, time.Time, error) {

	const functionID = 4

	// 1. Préparation des arguments (gestion des types spéciaux)
	args := make([]any, 6)
	args[0] = UserID      // p_user_id (ex: UUID)
	args[1] = MasterToken // p_master_token (ex: "master_token_string")
	if DeviceToken == "" {
		args[2] = nil
	} else {
		args[2] = DeviceToken // p_device_token (ex: "token_string")
	}
	if DeviceInfo == nil {
		args[3] = nil
	} else {
		// On transforme l'objet (map) en JSON string
		bytes, err := json.Marshal(DeviceInfo)
		if err != nil {
			fmt.Printf("⚠️ Warning FuncCreateSession: échec marshal DeviceInfo: %v\n", err)
			args[3] = "{}" // Envoie un JSON vide valide par sécurité
		} else {
			args[3] = string(bytes) // p_device_info (ex: JSON string)
		}
	}
	if len(IPHistory) == 0 {
		args[4] = nil
	} else {
		args[4] = pq.Array(IPHistory) // p_ip_history (ex: TEXT[])
	}
	args[5] = ExpiresAt // p_expires_at (ex: TIMESTAMP)

	// 2. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
	sqlStatement := `
		SELECT * FROM auth.func_create_session($1, $2, $3, $4, $5, $6)
	`

	// 3. Exécution via la connexion partagée du package 'db'
	var returnedID int
	var createdAt time.Time
	err := postgres.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedID, &createdAt)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("erreur lors de l'exécution de FuncCreateSession (ID %d): %w", functionID, err)
	}

	// 4. Retour du résultat
	return returnedID, createdAt, nil
}
func ProcUpdateSession(ID int64, MasterToken string, DeviceInfo any, DeviceToken string, IPHistory []string, ExpiresAt time.Time) error {

	const functionID = 5

	// 1. Préparation des arguments (Il faut 6 arguments, pas 5)
	args := make([]any, 6)
	args[0] = ID          // $1
	args[1] = MasterToken // $2

	// Gestion spéciale pour p_device_info (JSONB) -> $3
	if DeviceInfo == nil {
		args[2] = nil
	} else {
		// IMPORTANT : On convertit la MAP en STRING JSON pour Postgres
		bytes, err := json.Marshal(DeviceInfo)
		if err != nil {
			fmt.Printf("⚠️ Warning ProcUpdateSession: échec marshal DeviceInfo: %v\n", err)
			args[2] = "{}"
		} else {
			args[2] = string(bytes)
		}
	}

	// Gestion spéciale pour p_device_token (TEXT) -> $4
	if DeviceToken == "" {
		args[3] = nil
	} else {
		args[3] = DeviceToken
	}

	// Gestion spéciale pour p_ip_history (TEXT[]) -> $5
	// C'est l'argument qui manquait !
	if len(IPHistory) == 0 {
		args[4] = nil
	} else {
		args[4] = pq.Array(IPHistory)
	}

	args[5] = ExpiresAt // $6 (Décalé à la fin)

	// 2. Définition de la requête SQL (6 paramètres)
	sqlStatement := `
		CALL auth.proc_update_session($1, $2, $3, $4, $5, $6)
	`

	// 3. Exécution
	_, err := postgres.PostgresDB.Exec(sqlStatement, args...)
	if err != nil {
		return fmt.Errorf("erreur lors de l'exécution de ProcUpdateSession (ID %d): %w", functionID, err)
	}

	return nil
}
