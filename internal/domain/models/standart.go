package models

type ExternalLinks struct {
	Title string `json:"title" bson:"title" msgpack:"title"`
	URL   string `json:"url" bson:"url" msgpack:"url"`
}
