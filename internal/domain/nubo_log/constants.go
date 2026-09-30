package nubo_log

// Clés standards pour l'indexation structurée des logs
const (
	// Observabilité & Sécurité
	LogKeyTraceID   = "trace_id"
	LogKeyUserID    = "user_id"
	LogKeyIPAddress = "ip_address"
	LogKeySessionID = "session_id"

	// Métadonnées Métier
	LogKeyEntityType = "entity_type"
	LogKeyEntityID   = "entity_id"
	LogKeyAction     = "action"

	// Intégration des Erreurs
	LogKeyErrorCode  = "error_code"
	LogKeyHTTPStatus = "http_status"

	// Performance
	LogKeyLatency    = "latency_ms"
	LogKeyWorkerName = "worker_name"
	LogKeyShardID    = "shard_id"
)

// Constantes métiers pour garantir la propreté du paramètre Action()
const (
	ActionCreate     = "CREATE"
	ActionUpdate     = "UPDATE"
	ActionDelete     = "DELETE"
	ActionSoftDelete = "SOFT_DELETE"
	ActionHydrate    = "HYDRATE"
	ActionFanOut     = "FAN_OUT"
	ActionSync       = "SYNC"
	ActionEvict      = "EVICT"
)
