package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

func FuncLoadUser(ID int64, Username string, Email string, Phone string) (domain.UserRequest, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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

// scanPosts mutualise la logique d'itération et de scan des lignes (DRY).
// Elle lit les 16 colonnes (incluant view_count et vector) pour construire les PostRequests.
func scanPosts(rows *sql.Rows) ([]domain.PostRequest, error) {
	var posts []domain.PostRequest

	for rows.Next() {
		var p domain.PostRequest
		var location sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Content,
			pq.Array(&p.Hashtags),
			pq.Array(&p.Identifiers),
			pq.Array(&p.MediaIDs),
			&p.Visibility,
			&location,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.LikeCount,
			&p.CommentCount,
			&p.ViewCount,
			&p.HasMedia,
			pq.Array(&p.Vector),
			&p.VectorVersion,
		)

		if err != nil {
			fmt.Printf("⚠️ Erreur lors du scan d'un post : %v\n", err)
			continue // On ignore la ligne corrompue et on passe à la suivante
		}

		if location.Valid {
			p.Location = location.String
		}

		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur pendant l'itération des posts : %w", err)
	}

	return posts, nil
}

func FuncLoadPosts(postIDs []int64, limit int, offset int) ([]domain.PostRequest, error) {
	fmt.Println("FuncLoadPosts called with IDs count:", len(postIDs), "Limit:", limit, "Offset:", offset)

	// 1. Préparation de l'argument des IDs
	var pPostIDs any
	if len(postIDs) > 0 {
		pPostIDs = pq.Array(postIDs)
	} else {
		// Très important : si le tableau est vide, on passe nil.
		// Postgres recevra NULL, ce qui validera la condition "p_post_ids IS NULL" de ta fonction SQL.
		pPostIDs = nil
	}

	// 2. Requête SQL alignée sur ta fonction :
	// func_load_posts(p_user_id, p_post_ids, p_visibility, p_order_mode)
	// On gère la pagination avec LIMIT et OFFSET à l'extérieur de la fonction SQL
	sqlStatement := `
		SELECT * FROM content.func_load_posts(
			NULL,  -- p_user_id (NULL = tous les utilisateurs)
			$1,    -- p_post_ids (NULL ou tableau d'IDs)
			NULL,  -- p_visibility (NULL = déclenche le DEFAULT ARRAY[0, 1] du SQL)
			0      -- p_order_mode (0 = plus récents)
		)
		LIMIT $2 OFFSET $3
	`

	// 3. Exécution de la requête
	rows, err := postgres.PostgresDB.Query(sqlStatement, pPostIDs, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadPosts: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadPosts:", err)
		}
	}(rows)

	// NOUVEAU : Un seul appel remplace toute la boucle
	return scanPosts(rows)
}

// FuncLoadAllTags récupère tous les slugs actifs depuis la base de données.
func FuncLoadAllTags() ([]string, error) {
	fmt.Println("FuncLoadAllTags called")

	sqlStatement := `SELECT slug FROM content.func_load_all_tags()`

	rows, err := postgres.PostgresDB.Query(sqlStatement)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadAllTags: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadAllTags:", err)
		}
	}(rows)

	var tags []string

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err == nil {
			tags = append(tags, slug)
		} else {
			fmt.Printf("⚠️ Erreur lors du scan d'un tag : %v\n", err)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur pendant l'itération de FuncLoadAllTags: %w", err)
	}

	return tags, nil
}

// FuncLoadPostsPaginated récupère les posts par lots pour le démarrage à froid (Seeding).
// Cela empêche la saturation de la RAM lors de l'hydratation du cache.
func FuncLoadPostsPaginated(limit int, offset int) ([]domain.PostRequest, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.content, p.hashtags, p.identifiers, p.media_ids, 
			p.visibility, p.location, p.created_at, p.updated_at, p.like_count, 
			p.comment_count, p.view_count, p.has_media, p.vector, p.vector_version
		FROM content.posts p
		WHERE p.visibility != 2
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de FuncLoadPostsPaginated: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadPostsPaginated:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}

// FuncLoadRecentPosts récupère les posts créés dans l'intervalle de jours spécifié.
// Utilisé pour la synchronisation L2 (MongoDB) lors du démarrage à froid.
func FuncLoadRecentPosts(days int) ([]domain.PostRequest, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.content, p.hashtags, p.identifiers, p.media_ids, 
			p.visibility, p.location, p.created_at, p.updated_at, p.like_count, 
			p.comment_count, p.view_count, p.has_media, p.vector, p.vector_version
		FROM content.posts p
		WHERE p.visibility != 2
		AND p.created_at >= NOW() - ($1 || ' days')::interval
		ORDER BY p.created_at DESC
	`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("erreur FuncLoadRecentPosts: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadRecentPosts:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
