package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

func FuncLoadUser(ID int64, Username string, Email string, Phone string) (auth_models.UserPayload, error) {
	fmt.Println("FuncLoadUser called with:", ID, Username, Email, Phone)

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
	var res auth_models.UserPayload

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
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Println("🐘 POSTGRES : Aucune ligne trouvée (sql.ErrNoRows).")
			return auth_models.UserPayload{}, nil // On renvoie vide, pas d'erreur technique
		}
		fmt.Printf("❌ ERREUR SQL FuncLoadUser (Scan): %v\n", err)
		return auth_models.UserPayload{}, fmt.Errorf("erreur SQL LoadUser: %w", err)
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
