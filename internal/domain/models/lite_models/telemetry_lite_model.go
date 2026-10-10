package lite_models

type TelemetryProfileLite struct {
	Vector          []float32 `msgpack:"vector" json:"vector" bson:"vector"`
	TopTags         []string  `msgpack:"top_tags" json:"top_tags" bson:"top_tags"`
	ConfidenceScore float64   `msgpack:"confidence_score" json:"confidence_score" bson:"confidence_score"`
	TimestampMs     int64     `msgpack:"timestamp_ms" json:"timestamp_ms" bson:"timestamp_ms"`
}
