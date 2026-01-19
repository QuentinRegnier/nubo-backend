package postgres

import (
	"database/sql"
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

func FuncLoadUser(ID int, Username string, Email string, Phone string) (domain.UserRequest, error) {
	const functionID = 2

	// Args...
	args := make([]any, 4)
	args[0] = ID
	if ID == -1 {
		args[0] = nil
	}
	args[1] = Username
	if Username == "" {
		args[1] = nil
	}
	args[2] = Email
	if Email == "" {
		args[2] = nil
	}
	args[3] = Phone
	if Phone == "" {
		args[3] = nil
	}

	// 🕵️ DEBUG : On affiche la requête exacte
	fmt.Printf("\n🐘 POSTGRES QUERY : SELECT * FROM auth.func_load_user(%v, %v, '%v', %v)\n", args[0], args[1], args[2], args[3])

	sqlStatement := `SELECT * FROM auth.func_load_user($1, $2, $3, $4)`
	var res domain.UserRequest

	var birthdateRaw sql.NullString
	var profilePicID sql.NullInt64
	var banReason sql.NullString
	var banExpiresAt sql.NullTime
	var firstName sql.NullString
	var lastName sql.NullString
	var bio sql.NullString
	var location sql.NullString
	var school sql.NullString
	var work sql.NullString

	err := postgres.PostgresDB.QueryRow(sqlStatement, args...).Scan(
		&res.ID,
		&res.Username,
		&res.Email,
		&res.EmailVerified,
		&res.Phone,
		&res.PhoneVerified,
		&res.PasswordHash,
		&firstName,
		&lastName,
		&birthdateRaw,
		&res.Sex,
		&bio,
		&profilePicID,
		&res.Grade,
		&location,
		&school,
		&work,
		pq.Array(&res.Badges),
		&res.Desactivated,
		&res.Banned,
		&banReason,
		&banExpiresAt,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		// Si l'erreur est "no rows in result set", c'est que la BDD a renvoyé 0 ligne.
		if err == sql.ErrNoRows {
			fmt.Println("🐘 POSTGRES : Aucune ligne trouvée (sql.ErrNoRows).")
			return domain.UserRequest{}, nil // On renvoie vide, pas d'erreur technique
		}
		fmt.Printf("❌ ERREUR SQL FuncLoadUser (Scan): %v\n", err)
		return domain.UserRequest{}, fmt.Errorf("erreur SQL LoadUser: %w", err)
	}

	// 🕵️ DEBUG : On affiche ce qu'on a scanné
	fmt.Printf("🐘 POSTGRES SUCCÈS : ID=%d, Email='%s', Pass='%s'\n", res.ID, res.Email, res.PasswordHash)

	// ... (Le reste de la conversion date/nulls inchangé) ...
	if birthdateRaw.Valid {
		t, errTime := time.Parse(time.RFC3339, birthdateRaw.String)
		if errTime == nil {
			res.Birthdate = t
		} else {
			t2, errTime2 := time.Parse("02012006", birthdateRaw.String)
			if errTime2 == nil {
				res.Birthdate = t2
			}
		}
	}
	if profilePicID.Valid {
		res.ProfilePictureID = int(profilePicID.Int64)
	}
	if banReason.Valid {
		res.BanReason = banReason.String
	}
	if banExpiresAt.Valid {
		res.BanExpiresAt = banExpiresAt.Time
	}
	if firstName.Valid {
		res.FirstName = firstName.String
	}
	if lastName.Valid {
		res.LastName = lastName.String
	}
	if bio.Valid {
		res.Bio = bio.String
	}
	if location.Valid {
		res.Location = location.String
	}
	if school.Valid {
		res.School = school.String
	}
	if work.Valid {
		res.Work = work.String
	}

	return res, nil
}

func FuncLoadSession(ID int, UserId int, DeviceToken string, MasterToken string) (domain.SessionsRequest, error) {

	const functionID = 3

	// 1. Vérification que les champs sont non nuls
	if ID == -1 && UserId == -1 && DeviceToken == "" && MasterToken == "" {
		return domain.SessionsRequest{}, fmt.Errorf("erreur: champs requis manquants pour FuncLoadSession (ID %d)", functionID)
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
	var res domain.SessionsRequest
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
		if err == sql.ErrNoRows {
			// Ce n'est pas une erreur technique, juste qu'il n'y a pas de session.
			// On renvoie une structure vide et "pas d'erreur".
			return domain.SessionsRequest{}, nil
		}
		// ---------------------------------------------------------------

		return domain.SessionsRequest{}, fmt.Errorf("erreur lors de l'exécution de FuncLoadSession (ID %d): %w", functionID, err)
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
func FuncCreateSession(UserID int, MasterToken string, DeviceInfo any, DeviceToken string, IPHistory []string, ExpiresAt time.Time) (int, time.Time, error) {

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

func ProcUpdateSession(ID int, MasterToken string, DeviceInfo any, DeviceToken string, IPHistory []string, ExpiresAt time.Time) error {

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
