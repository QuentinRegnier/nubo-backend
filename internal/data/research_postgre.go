package data

import (
	"fmt"

	"github.com/lib/pq" // Nécessaire pour la conversion des ARRAY (ex: p_laws)

	// Remplacez 'votre_module_go' par le nom de votre module
	// (défini dans go.mod) pour que l'import de votre package 'db' fonctionne.
	"github.com/QuentinRegnier/nubo-backend/internal/db"
)

// Définissez vos constantes d'ID de fonction ici pour les rendre
// accessibles dans tout votre projet lors de l'appel à CallSQLFunction.
const (
	FuncLoadUser                   = 2
	ProcUpdateUser                 = 3
	ProcDeleteUser                 = 4
	FuncCreateUserSettings         = 5
	FuncLoadUserSettings           = 6
	ProcUpdateUserSettings         = 7
	ProcDeleteUserSettings         = 8
	FuncCreateSession              = 9
	FuncLoadSession                = 10
	ProcUpdateSession              = 11
	ProcDeleteSession              = 12
	FuncCreateRelation             = 13
	FuncLoadRelation               = 14
	ProcUpdateRelation             = 15
	ProcDeleteRelation             = 16
	FuncCreatePost                 = 17
	FuncLoadPost                   = 18
	ProcUpdatePost                 = 19
	ProcDeletePost                 = 20
	FuncCreateMedia                = 21
	FuncLoadMedia                  = 22
	ProcDeleteMedia                = 23
	FuncAddLike                    = 24
	FuncLoadLike                   = 25
	ProcRemoveLike                 = 26
	FuncCreateComment              = 27
	FuncLoadComment                = 28
	ProcUpdateComment              = 29
	ProcDeleteComment              = 30
	FuncCreateMessage              = 31
	FuncLoadMessage                = 32
	ProcUpdateMessage              = 33
	ProcDeleteMessage              = 34
	FuncCreateConversation         = 35
	FuncLoadConversation           = 36
	ProcUpdateConversation         = 37
	ProcDeleteConversation         = 38
	FuncCreateMembers              = 39
	FuncLoadMembers                = 40
	ProcUpdateMember               = 41
	ProcDeleteMember               = 42
	ProcUpdateUserConversationRole = 43
	FuncLoadReports                = 44
	ProcCreateReport               = 45
	ProcUpdateReport               = 46
)

/**
 * CallSQLFunction est une passerelle centralisée pour exécuter des fonctions
 * SQL définies dans PostgreSQL.
 *
 * Elle utilise la connexion partagée 'db.PostgresDB'.
 *
 * @param functionID - L'identifiant (constante) de la fonction à appeler.
 * @param params - La liste des arguments à passer à la fonction SQL.
 * @return interface{} - Le résultat retourné par la fonction SQL (ex: un ID, un booléen).
 * @return error - Une erreur si l'appel échoue.
 */
func CallSQLFunction(functionID int, params ...interface{}) (interface{}, error) {

	// Vérifie si la connexion BDD est bien initialisée
	if db.PostgresDB == nil {
		return nil, fmt.Errorf("la connexion à PostgreSQL (db.PostgresDB) n'est pas initialisée")
	}

	switch functionID {
	case FuncLoadUser:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncLoadUser (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[1] = params[1] // p_username (ex: "johndoe")
		args[2] = params[2] // p_email (ex: "john@example.com")
		args[3] = params[3] // p_phone (ex: "+1234567890" ou nil)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_load_user($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadUser (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteUser:
		// 1. Vérification du nombre de paramètres
		if len(params) != 7 {
			return nil, fmt.Errorf("ProcDeleteUser (ID %d) attend 7 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 7)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[1] = params[1] // p_username (ex: "johndoe")
		args[2] = params[2] // p_email (ex: "john@example.com")
		args[3] = params[3] // p_phone (ex: "+1234567890" ou nil)
		args[4] = params[4] // p_ban (bool)
		args[5] = params[5] // p_ban_reason (string ou nil)
		args[6] = params[6] // p_ban_expires_at (timestamp ou nil)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_delete_user($1, $2, $3 , $4, $5, $6, $7)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteUser (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Utilisateur supprimé", nil
	case ProcUpdateUser:
		// 1. Vérification du nombre de paramètres
		if len(params) != 15 {
			return nil, fmt.Errorf("ProcUpdateUser (ID %d) attend 15 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 15)
		args[0] = params[0]   // p_user_id (ex: UUID)
		args[1] = params[1]   // p_username (ex: "johndoe")
		args[2] = params[2]   // p_email (ex: "john@example.com")
		args[3] = params[3]   // p_email_verified (bool)
		args[4] = params[4]   // p_phone (ex: "+1234567890" ou nil)
		args[5] = params[5]   // p_phone_verified (bool)
		args[6] = params[6]   // p_password_hash (ex: "hashed_password")
		args[7] = params[7]   // p_first_name (ex: "John" ou nil)
		args[8] = params[8]   // p_last_name (ex: "Doe" ou nil)
		args[9] = params[9]   // p_profile_picture_id (ex: UUID ou nil)
		args[10] = params[10] // p_location (ex: "City, Country" ou nil)
		args[11] = params[11] // p_school (ex: "University Name" ou nil)
		args[12] = params[12] // p_work (ex: "Company Name" ou nil)
		args[13] = params[13] // p_desactivated (ex: false)
		args[14] = params[14] // p_updated_at (timestamp)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_update_user($1, $2, $3 , $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateUser (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Utilisateur mis à jour", nil
	case FuncCreateUserSettings:
		// 1. Vérification du nombre de paramètres
		if len(params) != 5 {
			return nil, fmt.Errorf("FuncCreateUserSettings (ID %d) attend 5 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 5)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[3] = params[3] // p_language (ex: "en" ou "fr")
		args[4] = params[4] // p_theme (ex: "dark" ou "light")

		// Gestion spéciale pour p_privacy (JSONB)
		if params[1] == nil {
			args[1] = nil
		} else {
			args[1] = params[1]
		}
		// Gestion spéciale pour p_notifications (JSONB)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = params[2]
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_create_user_settings($1, $2, $3, $4, $5)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateUserSettings (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadUserSettings:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("FuncLoadUserSettings (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_load_user_settings($1, $2)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadUserSettings (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteUserSettings:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeleteUserSettings (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_delete_user_settings($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteUserSettings (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Paramètres utilisateur supprimés", nil
	case ProcUpdateUserSettings:
		// 1. Vérification du nombre de paramètres
		if len(params) != 6 {
			return nil, fmt.Errorf("ProcUpdateUserSettings (ID %d) attend 6 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 6)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[4] = params[4] // p_language (ex: TEXT ou nil)
		args[5] = params[5] // p_theme (ex: TEXT ou nil)

		// Gestion spéciale pour p_privacy (JSONB)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = params[2]
		}
		// Gestion spéciale pour p_notifications (JSONB)
		if params[3] == nil {
			args[3] = nil
		} else {
			args[3] = params[3]
		}

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_update_user_settings($1, $2, $3, $4, $5, $6)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateUserSettings (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Paramètres utilisateur mis à jour", nil
	case FuncCreateSession:
		// 1. Vérification du nombre de paramètres
		if len(params) != 6 {
			return nil, fmt.Errorf("FuncCreateSession (ID %d) attend 6 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 6)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[1] = params[1] // p_refresh_token (ex: "refresh_token_string")
		args[4] = params[4] // p_ip_history (ex: ARRAY ou nil)
		args[5] = params[5] // p_expires_at (ex: timestamp ou nil)

		// Gestion spéciale pour p_device_info (JSONB)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = params[2]
		}
		// Gestion spéciale pour p_device_token (TEXT)
		if params[3] == nil {
			args[3] = nil
		} else {
			args[3] = params[3]
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_create_session($1, $2, $3, $4, $5, $6)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateSession (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadSession:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncLoadSession (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_device_token (ex: "device_token_string" ou nil)
		args[3] = params[3] // p_refresh_token (ex: "refresh_token_string")

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_load_sessions($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadSession (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteSession:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("ProcDeleteSession (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_device_token (ex: "device_token_string" ou nil)
		args[3] = params[3] // p_refresh_token (ex: "refresh_token_string")

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_delete_session($1, $2, $3 , $4)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteSession (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Session supprimée", nil
	case ProcUpdateSession:
		// 1. Vérification du nombre de paramètres
		if len(params) != 5 {
			return nil, fmt.Errorf("ProcUpdateSession (ID %d) attend 5 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 5)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_refresh_token (ex: "refresh_token_string")
		args[4] = params[4] // p_expires_at (ex: timestamp ou nil)

		// Gestion spéciale pour p_device_info (JSONB)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = params[2]
		}
		// Gestion spéciale pour p_device_token (TEXT)
		if params[3] == nil {
			args[3] = nil
		} else {
			args[3] = params[3]
		}

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_update_session($1, $2, $3 , $4, $5)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateSession (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Session mise à jour", nil
	case FuncCreateRelation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("FuncCreateRelation (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_primary_id (ex: UUID)
		args[1] = params[1] // p_secondary_id (ex: UUID)
		args[2] = params[2] // p_state (ex: INT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_create_relation($1, $2, $3)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateRelation (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadRelation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncLoadRelation (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_primary_id (ex: UUID)
		args[2] = params[2] // p_secondary_id (ex: UUID)
		args[3] = params[3] // p_state (ex: INT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT auth.func_load_relation($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadRelation (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteRelation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeleteRelation (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_target_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_delete_relation($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteRelation (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Relation supprimée", nil
	case ProcUpdateRelation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcUpdateRelation (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_primary_id (ex: UUID)
		args[1] = params[1] // p_secondary_id (ex: UUID)
		args[2] = params[2] // p_state (ex: INT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL auth.proc_update_relation($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateRelation (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Relation mise à jour", nil
	case FuncCreatePost:
		// 1. Vérification du nombre de paramètres
		if len(params) != 5 {
			return nil, fmt.Errorf("FuncCreatePost (ID %d) attend 5 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 5)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[1] = params[1] // p_content (ex: TEXT)
		args[2] = params[3] // p_visibility (ex: INT)
		args[3] = params[4] // p_location (ex: TEXT ou nil)

		// Gestion spéciale pour p_media_ids (ARRAY)
		if params[2] == nil {
			args[4] = nil
		} else {
			args[4] = pq.Array(params[2])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_create_post($1, $2, $3, $4, $5)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreatePost (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadPost:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncLoadPost (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_user_id (ex: UUID)
		args[2] = params[2] // p_visibility (ex: INT)
		args[3] = params[3] // p_order_mode (ex: INT)

		// Gestion spéciale pour p_post_ids (ARRAY)
		if params[1] == nil {
			args[1] = nil
		} else {
			args[1] = pq.Array(params[1])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_load_post($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadPost (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeletePost:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeletePost (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_delete_post($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeletePost (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Post supprimé", nil
	case ProcUpdatePost:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcUpdatePost (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_post_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_content (ex: TEXT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_update_post($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdatePost (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Post mis à jour", nil
	case FuncCreateMedia:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("FuncCreateMedia (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_owner_id (ex: UUID)
		args[1] = params[1] // p_storage_path (ex: TEXT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_create_media($1, $2)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateMedia (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadMedia:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("FuncLoadMedia (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_owner_id (ex: UUID)
		args[2] = params[2] // p_order_mode (ex: INT)

		// Gestion spéciale pour p_media_ids (ARRAY)
		if params[1] == nil {
			args[1] = nil
		} else {
			args[1] = pq.Array(params[1])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_load_media($1, $2, $3)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadMedia (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteMedia:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeleteMedia (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_owner_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_delete_media($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteMedia (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Media supprimée", nil
	case FuncAddLike:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("FuncAddLike (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_targer_type (ex: INT)
		args[1] = params[1] // p_target_id (ex: UUID)
		args[2] = params[2] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_add_like($1, $2, $3)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncAddLike (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadLike:
		// 1. Vérification du nombre de paramètres
		if len(params) != 6 {
			return nil, fmt.Errorf("FuncLoadLike (ID %d) attend 6 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 6)
		args[0] = params[0] // p_target_type (ex: INT)
		args[2] = params[2] // p_target_id (ex: UUID)
		args[3] = params[3] // p_user_id (ex: UUID)
		args[4] = params[4] // p_limit (ex: INT)
		args[5] = params[5] // p_order_mode (ex: INT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_load_likes($1, $2, $3, $4, $5, $6)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadLike (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcRemoveLike:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcRemoveLike (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_target_type (ex: INT)
		args[1] = params[1] // p_target_id (ex: UUID)
		args[2] = params[2] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_remove_like($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcRemoveLike (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Like supprimé", nil
	case FuncCreateComment:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("FuncCreateComment (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 6)
		args[0] = params[0] // p_post_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_content (ex: TEXT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_create_comment($1, $2, $3)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateComment (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadComment:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncLoadComment (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_post_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_limit (ex: INT)
		args[3] = params[3] // p_order_mode (ex: INT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT content.func_load_comments($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadComment (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteComment:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcDeleteComment (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_id (ex: UUID)
		args[1] = params[1] // p_post_id (ex: UUID)
		args[2] = params[2] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_delete_comment($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteComment (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Commentaire supprimé", nil
	case ProcUpdateComment:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcUpdateComment (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_comment_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_content (ex: TEXT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL content.proc_update_comment($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateComment (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Commentaire mis à jour", nil
	case FuncCreateMessage:
		// 1. Vérification du nombre de paramètres
		if len(params) != 6 {
			return nil, fmt.Errorf("FuncCreateMessage (ID %d) attend 6 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 6)
		args[0] = params[0] // p_conversation_id (ex: UUID)
		args[1] = params[1] // p_sender_id (ex: UUID)
		args[2] = params[2] // p_content (ex: TEXT)
		args[4] = params[4] // p_message_type (ex: INT)
		args[5] = params[5] // p_visibility (ex: INT)

		// Gestion spéciale pour p_attachlents (ARRAY)
		if params[3] == nil {
			args[3] = nil
		} else {
			args[3] = pq.Array(params[3])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_create_message($1, $2, $3, $4, $5, $6)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateMessage (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadMessage:
		// 1. Vérification du nombre de paramètres
		if len(params) != 1 {
			return nil, fmt.Errorf("FuncLoadMessage (ID %d) attend 1 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 1)
		args[0] = params[0] // p_conversation_id (ex: UUID)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_load_message($1)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadMessage (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteMessage:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeleteMessage (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[1] = params[1] // p_user_id (ex: UUID)

		// Gestion spéciale pour p_message_ids (ARRAY)
		if params[0] == nil {
			args[0] = nil
		} else {
			args[0] = pq.Array(params[0])
		}

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_delete_message($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteMessage (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Message supprimé", nil
	case ProcUpdateMessage:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcUpdateMessage (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_message_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_content (ex: TEXT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_update_message($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateMessage (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Message mis à jour", nil
	case FuncCreateConversation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("FuncCreateConversation (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_type (ex: INT)
		args[1] = params[1] // p_title (ex: TEXT)

		// Gestion spéciale pour p_laws (ARRAY)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = pq.Array(params[2])
		}
		// Gestion spéciale pour p_members (ARRAY)
		if params[3] == nil {
			args[3] = nil
		} else {
			args[3] = pq.Array(params[3])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_create_conversation($1, $2, $3, $4)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateConversation (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadConversation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 1 {
			return nil, fmt.Errorf("FuncLoadConversation (ID %d) attend 1 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 1)
		args[0] = params[0] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_load_conversation($1)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadConversation (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteConversation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 1 {
			return nil, fmt.Errorf("ProcDeleteConversation (ID %d) attend 1 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 1)
		args[0] = params[0] // p_conversation_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_delete_conversation($1)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteConversation (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Conversation supprimée", nil
	case ProcUpdateConversation:
		// 1. Vérification du nombre de paramètres
		if len(params) != 5 {
			return nil, fmt.Errorf("ProcUpdateConversation (ID %d) attend 5 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 5)
		args[0] = params[0] // p_conversation_id (ex: UUID)
		args[1] = params[1] // p_title (ex: TEXT)
		args[2] = params[2] // p_last_message_id (ex: UUID ou nil)
		args[3] = params[3] // p_state (ex: INT)

		// Gestion spéciale pour p_laws (ARRAY)
		if params[4] == nil {
			args[4] = nil
		} else {
			args[4] = pq.Array(params[4])
		}

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_update_conversation($1, $2, $3 , $4, $5)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateConversation (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Conversation mise à jour", nil
	case FuncCreateMembers:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("FuncCreateMembers (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_conversation_id (ex: UUID)

		// Gestion spéciale pour p_user_ids (ARRAY)
		if params[1] == nil {
			args[1] = nil
		} else {
			args[1] = pq.Array(params[1])
		}
		// Gestion spéciale pour p_roles (ARRAY)
		if params[2] == nil {
			args[2] = nil
		} else {
			args[2] = pq.Array(params[2])
		}

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_create_members($1, $2, $3)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncCreateMembers (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case FuncLoadMembers:
		// 1. Vérification du nombre de paramètres
		if len(params) != 1 {
			return nil, fmt.Errorf("FuncLoadMembers (ID %d) attend 1 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 1)
		args[0] = params[0] // p_conversation_id (ex: UUID)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT messaging.func_load_members($1)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadMembers (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcDeleteMember:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcDeleteMember (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_conversation_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_delete_member($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcDeleteMember (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Member supprimé", nil
	case ProcUpdateMember:
		// 1. Vérification du nombre de paramètres
		if len(params) != 4 {
			return nil, fmt.Errorf("ProcUpdateMember (ID %d) attend 4 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 4)
		args[0] = params[0] // p_conversation_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)
		args[2] = params[2] // p_role (ex: INT)
		args[3] = params[3] // p_unread_count (ex: INT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_update_member($1, $2, $3, $4)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateMember (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Member mis à jour", nil
	case ProcUpdateUserConversationRole:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("ProcUpdateUserConversationRole (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_conversation_id (ex: UUID)
		args[1] = params[1] // p_user_id (ex: UUID)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL messaging.proc_update_user_conversation_role($1, $2)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateUserProfile (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Rôle de l'utilisateur mis à jour", nil
	case FuncLoadReports:
		// 1. Vérification du nombre de paramètres
		if len(params) != 2 {
			return nil, fmt.Errorf("FuncLoadReports (ID %d) attend 2 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 2)
		args[0] = params[0] // p_state (ex: INT)
		args[1] = params[1] // p_limit (ex: INT)

		// 3. Définition de la requête SQL (TOUJOURS paramétrée pour éviter l'injection SQL)
		sqlStatement := `
			SELECT moderation.func_load_reports($1, $2)
		`

		// 4. Exécution via la connexion partagée du package 'db'
		//    Nous utilisons QueryRow car la fonction SQL retourne une seule valeur (le UUID)
		var returnedUUID string
		err := db.PostgresDB.QueryRow(sqlStatement, args...).Scan(&returnedUUID)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadReports (ID %d): %w", functionID, err)
		}

		// 5. Retour du résultat
		return returnedUUID, nil
	case ProcCreateReport:
		// 1. Vérification du nombre de paramètres
		if len(params) != 5 {
			return nil, fmt.Errorf("ProcCreateReport (ID %d) attend 5 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 5)
		args[0] = params[0] // p_actor_id (ex: UUID)
		args[1] = params[1] // p_target_type (ex: INT)
		args[2] = params[2] // p_target_id (ex: UUID)
		args[3] = params[3] // p_reason (ex: TEXT)
		args[4] = params[4] // p_details (ex: TEXT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL moderation.proc_create_report($1, $2, $3, $4, $5)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcCreateReport (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Rapport créé", nil
	case ProcUpdateReport:
		// 1. Vérification du nombre de paramètres
		if len(params) != 3 {
			return nil, fmt.Errorf("ProcUpdateReport (ID %d) attend 3 paramètres, mais en a reçu %d", functionID, len(params))
		}

		// 2. Préparation des arguments (gestion des types spéciaux)
		args := make([]interface{}, 3)
		args[0] = params[0] // p_report_id (ex: UUID)
		args[1] = params[1] // p_new_state (ex: INT)
		args[2] = params[2] // p_new_rationale (ex: TEXT)

		// 3. Définition de la requête SQL
		sqlStatement := `
			CALL moderation.proc_update_report($1, $2, $3)
		`

		// 4. Exécution avec tous les paramètres
		//    On utilise Exec() et on "déplie" le slice 'params'
		_, err := db.PostgresDB.Exec(sqlStatement, args...)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'exécution de ProcUpdateReport (ID %d): %w", functionID, err)
		}

		// 6. Retour du succès
		return "Rapport mis à jour", nil
	default:
		return nil, fmt.Errorf("ID de fonction SQL inconnu: %d", functionID)
	}
}
