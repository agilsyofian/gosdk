package http

import (
	"github.com/agilsyofian/gosdk/example/modules/auth/model"
)

func loginRequestMapper(req LoginPassword) (res model.Login) {
	res.Username = req.Username
	res.Password = req.Password
	return res
}

func loginResponseMapper(model model.Token) (res LoginResponse) {
	return LoginResponse{
		AccessToken:  model.AccessToken,
		RefreshToken: model.RefreshToken,
	}
}

func tokenRefreshRequestMapper(req TokenRefreshRequest) (res model.TokenRefresh) {
	res.Token = req.Token
	return res
}

func tokenRefreshResponseMapper(model model.TokenRefresh) (res LoginResponse) {
	return LoginResponse{
		AccessToken:  model.AccessToken,
		RefreshToken: model.RefreshToken,
	}
}
