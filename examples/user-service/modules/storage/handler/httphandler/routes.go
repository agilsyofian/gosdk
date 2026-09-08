package httphandler

import (
	"github.com/agilsyofian/gosdk/transport/server/http"
	serverHttp "user-service/app/server/httpserver"
)

type StorageRouteRegistrar struct {
	handler IStorageHandler
}

func NewRouteRegistrar(handler IStorageHandler) serverHttp.RouteRegistrar {
	return &StorageRouteRegistrar{
		handler: handler,
	}
}

func (r *StorageRouteRegistrar) RegisterRoutes(s *http.Server) {
	storageGroup := s.Router().Group("/v1/storage")
	{
		storageGroup.POST("/upload", r.handler.Upload)
		storageGroup.GET("/download/:filename", r.handler.Download)
		storageGroup.GET("/url/:filename", r.handler.PresignedURL)
	}
}
