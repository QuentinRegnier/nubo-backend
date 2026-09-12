package lite_models

// CommunityLiteRequest représente les données compressées stockées dans le Speed Cache Redis (MsgPack)
type CommunityLiteRequest struct {
	ID               int64  `msgpack:"id"`
	Name             string `msgpack:"name"`
	ProfilePictureID int64  `msgpack:"profile_picture_id"`
	Description      string `msgpack:"description"`
	MemberCount      int    `msgpack:"member_count"`
}
