package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE NOTIFICATIONS
// ############################################################################

const (
	// Types d'événements de notification supportés par le système Push FCM et les WebSockets
	EventPostLiked        = "post_liked"
	EventCommentLiked     = "comment_liked"
	EventCommentAdded     = "comment_added"
	EventRelationFollowed = "relation_followed"
	EventFriendshipEst    = "friendship_established"
	EventGroupInvited     = "group_invited"

	// Nom de la file d'attente asynchrone Redis pour les Push Firebase
	WorkerQueueFirebase = "firebase"

	MaxZsetNotification = 100
)
