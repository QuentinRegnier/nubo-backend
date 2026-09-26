package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE RELATIONS
// ############################################################################

const (
	// États de relation entre deux utilisateurs
	RelationStateBlocked = -1 // Utilisateur bloqué
	RelationStateNone    = 0  // Aucune relation
	RelationStateFollow  = 1  // Abonnement (Follower)
	RelationStateFriend  = 2  // Amitié validée

	// Niveaux de confidentialité (HideConnections)
	ConnectionsVisibilityPublic    = 0 // Visible par tout le monde
	ConnectionsVisibilityFollowers = 1 // Visible uniquement par les abonnés
	ConnectionsVisibilityFriends   = 2 // Visible uniquement par les amis
	ConnectionsVisibilityPrivate   = 3 // Visible par personne
)
