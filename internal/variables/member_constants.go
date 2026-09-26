package variables

// ============================================================================
// PARAMÈTRES ET CONSTANTES DU DOMAINE MEMBRES (MESSAGING.MEMBERS)
// ============================================================================

const (
	// Rôles des participants au sein d'une conversation ou communauté
	MemberRolePendingApproval = -4 // Rejeté / Demande refusée
	MemberRolePending         = -3 // En attente d'approbation (Candidature communauté)
	MemberRoleBanned          = -2 // Banni définitivement du groupe
	MemberRoleLeft            = -1 // A volontairement quitté la conversation
	MemberRoleNormal          = 0  // Membre classique
	MemberRoleAdmin           = 1  // Administrateur (Gestion des membres et messages)
	MemberRoleOwner           = 2  // Propriétaire (Gestion intégrale et transfert)

	// Plafond d'épingles par utilisateur
	MaxPinnedConversations = 3
)
