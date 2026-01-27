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

func FuncLoadUser(ID int64, Username string, Email string, Phone string) (domain.UserRequest, error) {
	fmt.Println("FuncLoadUser called with:", ID, Username, Email, Phone)
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
		res.ProfilePictureID = profilePicID.Int64
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
func FuncLoadSession(ID int64, UserId int64, DeviceToken string, MasterToken string) (domain.SessionsRequest, error) {
	fmt.Println("FuncLoadSession called with:", ID, UserId, DeviceToken, MasterToken)
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
