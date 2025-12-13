package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/tools"
	"github.com/lib/pq"
)

// FuncCreateUser accepte maintenant directement le modèle RegisterRequest.
// profilePictureID est passé à part car req.ProfilePicture contient du Base64, pas l'UUID.
func FuncCreateUser(req tools.UserRequest, sessions tools.SessionsRequest) (int, time.Time, time.Time, int, time.Time, error) {

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
	args[19] = sessions.RefreshToken
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
	err := PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID, &createdAtUser, &updatedAtUser, &returnedSessionUUID, &createdAtSession)
	if err != nil {
		return 0, time.Time{}, time.Time{}, 0, time.Time{}, fmt.Errorf("erreur FuncCreateUser (ID %d): %w", functionID, err)
	}

	return returnedUUID, createdAtUser, updatedAtUser, returnedSessionUUID, createdAtSession, err
}

func FuncLoadUser(ID int, Username string, Email string, Phone string) (tools.UserRequest, error) {

	const functionID = 2

	// 1. Vérification que les champs sont non nuls
	if ID == -1 && Username == "" && Email == "" && Phone == "" {
		return tools.UserRequest{}, fmt.Errorf("erreur: champs requis manquants pour FuncLoadUser (ID %d)", functionID)
	}

	// 2. Préparation des arguments (gestion des types spéciaux)
	args := make([]any, 4)
	args[0] = ID       // p_user_id (ex: UUID)
	args[1] = Username // p_username (ex: "johndoe")
	args[2] = Email    // p_email (ex: "john@example.com")
	args[3] = Phone    // p_phone (ex: "+1234567890" ou nil)

	if ID == -1 {
		args[0] = nil
	}
	if Username == "" {
		args[1] = nil
	}
	if Email == "" {
		args[2] = nil
	}
	if Phone == "" {
		args[3] = nil
	}

	// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
	sqlStatement := `
		SELECT auth.func_load_user($1, $2, $3, $4)
	`

	// 4. Exécution via la connexion partagée du package 'db'
	//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
	var res tools.UserRequest

	err := PostgresDB.QueryRow(sqlStatement, args...).Scan(&res.ID, &res.Username, &res.Email, &res.EmailVerified, &res.Phone, &res.PhoneVerified, &res.PasswordHash, &res.FirstName, &res.LastName, &res.Birthdate, &res.Sex, &res.Bio, &res.ProfilePictureID, &res.Grade, &res.Location, &res.School, &res.Work, &res.Badges, &res.Desactivated, &res.Banned, &res.BanReason, &res.BanExpiresAt, &res.CreatedAt, &res.UpdatedAt)
	if err != nil {
		return tools.UserRequest{}, fmt.Errorf("erreur lors de l'exécution de FuncLoadUser (ID %d): %w", functionID, err)
	}

	// 5. Retour du résultat
	return res, nil
}

func FuncLoadSession(ID int, DeviceToken string) (tools.SessionsRequest, error) {

	const functionID = 3

	// 1. Vérification que les champs sont non nuls
	if ID == -1 && DeviceToken == "" {
		return tools.SessionsRequest{}, fmt.Errorf("erreur: champs requis manquants pour FuncLoadSession (ID %d)", functionID)
	}

	// 2. Préparation des arguments (gestion des types spéciaux)
	args := make([]any, 2)
	args[0] = ID          // p_session_id (ex: UUID)
	args[1] = DeviceToken // p_device_token (ex: "token_string")

	if ID == -1 {
		args[0] = nil
	}
	if DeviceToken == "" {
		args[1] = nil
	}

	// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
	sqlStatement := `
		SELECT auth.func_load_session($1, $2)
	`

	// 4. Exécution via la connexion partagée du package 'db'
	//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
	var res tools.SessionsRequest

	var deviceInfoBytes []byte

	err := PostgresDB.QueryRow(sqlStatement, args...).Scan(&res.ID, &res.UserID, &res.RefreshToken, &res.DeviceToken, &deviceInfoBytes, &res.IPHistory, &res.CreatedAt, &res.ExpiresAt)
	if err != nil {
		return tools.SessionsRequest{}, fmt.Errorf("erreur lors de l'exécution de FuncLoadSession (ID %d): %w", functionID, err)
	}

	if len(deviceInfoBytes) > 0 {
		_ = json.Unmarshal(deviceInfoBytes, &res.DeviceInfo)
	}

	// 5. Retour du résultat
	return res, nil
}
