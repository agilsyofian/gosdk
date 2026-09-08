package httphandler

import (
	"auth-service/module/auth/domain"
)

func loginRequestMapper(req LoginPassword) (res domain.Login) {
	res.Username = req.Username
	res.Password = req.Password
	return res
}

func loginResponseMapper(m domain.Token) (res LoginResponse) {
	return LoginResponse{
		AccessToken:  m.AccessToken,
		RefreshToken: m.RefreshToken,
	}
}

func tokenRefreshRequestMapper(req TokenRefreshRequest) (res domain.RefreshTokenCmd) {
	res.RefreshToken = req.Token
	return res
}

func tokenRefreshResponseMapper(m domain.Token) (res LoginResponse) {
	return LoginResponse{
		AccessToken:  m.AccessToken,
		RefreshToken: m.RefreshToken,
	}
}
