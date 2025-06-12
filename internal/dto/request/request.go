package request

type RegisterRequest struct {
	Telephone string `json:"telephone"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname"`
	SmsCode   string `json:"sms_code"`
}

type LoginRequest struct {
	Telephone string `json:"telephone"`
	Password  string `json:"password"`
}

type SmsLoginRequest struct {
	Telephone string `json:"telephone"`
	SmsCode   string `json:"sms_code"`
}

type UpdateUserInfoRequest struct {
	Uuid      string `json:"uuid"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	Birthday  string `json:"birthday"`
	Signature string `json:"signature"`
	Avatar    string `json:"avatar"`
}

type GetUserInfoListRequest struct {
	OwnerId string `json:"owner_id"`
}

type AbleUsersRequest struct {
	UuidList []string `json:"uuid_list"`
	IsAdmin  int8     `json:"is_admin"`
}

type GetUserInfoRequest struct {
	Uuid string `json:"uuid"`
}

type SendSmsCodeRequest struct {
	Telephone string `json:"telephone"`
}
