package lite_models

type UserLiteRequest struct {
	ID                     int64    `bson:"id" json:"id"`
	Username               string   `bson:"username" json:"username"`
	FirstName              string   `bson:"first_name" json:"first_name"`
	LastName               string   `bson:"last_name" json:"last_name"`
	ProfilePictureID       int64    `bson:"profile_picture_id" json:"profile_picture_id"`
	Bio                    string   `bson:"bio" json:"bio"`
	Grade                  int      `bson:"grade" json:"grade"`
	Badges                 []string `bson:"badges" json:"badges"`
	ConversationPermission int      `bson:"conversation_permission" json:"conversation_permission"` // 0=Tout le monde, 1=Abonnés, 2=Amis
	AddGroupPermission     bool     `bson:"add_group_permission" json:"add_group_permission"`       // true=Auto, false=Invitation
}
