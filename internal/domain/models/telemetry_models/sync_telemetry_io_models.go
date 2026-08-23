package telemetry_models

type SyncTelemetryInput struct {
	UserID  int64                `json:"-"`
	Payload SyncTelemetryPayload `json:"payload"`
}

type SyncTelemetryOutput struct {
	Status        string    `json:"status"`
	NeedUpdate    bool      `json:"need_update"`
	ServerVector  []float32 `json:"server_vector"`
	ServerTags    []string  `json:"server_tags"`
	ServerUpdated int64     `json:"server_updated"`
}
