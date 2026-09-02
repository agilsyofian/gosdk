package http

import (
	"net/http"

	"github.com/agilsyofian/gosdk/example/modules/user/service"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

type IUserHandler interface {
	UserDetail(c *gin.Context)
	Update(c *gin.Context)
}

type UserHandler struct {
	service service.IUserService
}

func NewHandler(service service.IUserService) IUserHandler {
	return &UserHandler{
		service: service,
	}
}

func (h *UserHandler) UserDetail(c *gin.Context) {
	ctx := c.Request.Context()
	res, err := h.service.Detail(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.New(ctx, UserToUserDetail(res)).Success())
}

func (h *UserHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	_, err := h.service.Update(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.New(ctx, nil).Success())
}
