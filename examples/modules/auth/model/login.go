package model

type Login struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	DeviceIdentifier string `json:"deviceIdentifier"`
}

type Token struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type TokenRefresh struct {
	Token        string `json:"token"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
