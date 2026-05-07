package domain

// RoomMapping links an internal room type to an external channel's room type.
type RoomMapping struct {
	ID                 string  `json:"id"`
	OrgID              string  `json:"org_id"`
	InternalRoomTypeID string  `json:"internal_room_type_id"`
	ExternalID         string  `json:"external_id"`
	ChannelID          string  `json:"channel_id"`
	MappingVersion     int     `json:"mapping_version"`
	ConfidenceScore    float64 `json:"confidence_score"`
}
