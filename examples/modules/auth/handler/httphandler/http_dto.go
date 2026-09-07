package httphandler

type LoginPassword struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type TokenRefreshRequest struct {
	Token string `json:"token" binding:"required"`
}
