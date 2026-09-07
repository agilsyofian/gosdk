package httphandler

import (
	"net/http"

	"github.com/agilsyofian/gosdk/example/modules/auth/service"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

type IAuthHandler interface {
	Certificates(c *gin.Context)
	Password(c *gin.Context)
	Logout(c *gin.Context)
	TokenRefresh(c *gin.Context)
}

type AuthHandler struct {
	service service.IAuthService
}

func New(service service.IAuthService) IAuthHandler {
	return &AuthHandler{
		service: service,
	}
}

func (h *AuthHandler) Certificates(c *gin.Context) {
	ctx := c.Request.Context()
	res, err := h.service.Certificates(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AuthHandler) Password(c *gin.Context) {
	var req LoginPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}
	ctx := c.Request.Context()

	res, err := h.service.LoginPassword(ctx, loginRequestMapper(req))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.New(ctx, loginResponseMapper(res)).Success())
}

func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	err := h.service.Logout(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.New(ctx, nil).Success())
}

func (h *AuthHandler) TokenRefresh(c *gin.Context) {
	var req TokenRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		return
	}
	ctx := c.Request.Context()

	res, err := h.service.TokenRefresh(ctx, tokenRefreshRequestMapper(req))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.New(ctx, tokenRefreshResponseMapper(res)).Success())
}
