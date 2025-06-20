package request

type DeleteSessionRequest struct {
	OwnerId   string `json:"owner_id"`
	ReceiveId string `json:"Receive_id"`
	SessionId string `json:"session_id"`
}
