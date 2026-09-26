package variables

// ============================================================================
// PARAMÈTRES ET CONSTANTES DU DOMAINE CONVERSATIONS & MESSAGERIE
// ============================================================================

const (
	// Types de conversation
	ConversationTypeDirect        = 0 // Message Privé (MP)
	ConversationTypeGroup         = 1 // Groupe standard
	ConversationTypeCommunityPriv = 2 // Communauté privée
	ConversationTypeCommunityPub  = 3 // Communauté publique

	ConversationStateAll           = 0
	ConversationStatePrivateDelete = -1
	ConversationStateGroupDelete   = -2

	MaxPinConversation = 3

	MaxActiveParticipantsForBroadcast = 50
	MaxZsetInbox                      = 100

	// Limites et seuils d'autorisation
	CommunityMinCreationGrade = 2 // Grade minimum (Partenaire/Collaborateur) pour créer une communauté
	MaxPublicCommunitiesColl  = 1 // Limite de communautés gérées pour un compte de grade 2

	// États globaux d'une conversation
	ConversationStateActive   = 0
	ConversationStateArchived = 1
)
