package variables

import "time"

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

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER DE PUSH NOTIFICATIONS
// ============================================================================
const (
	PushWorkerQueueName      = "worker:queue:firebase"
	PushWorkerBLPopTimeout   = 2 * time.Second
	PushWorkerDefaultTitle   = "Nubo"
	PushWorkerDefaultBody    = "Nouvelle notification"
	PushWorkerDefaultSound   = "default"
	PushWorkerDefaultChannel = "default_channel"
)
