package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/example/app/config"
	authHandler "github.com/agilsyofian/gosdk/example/modules/auth/handler/http"
	userHandler "github.com/agilsyofian/gosdk/example/modules/user/handler/http"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/agilsyofian/gosdk/transport/server/http"
)

type IHttpHandler interface {
	SetupRouter()
}

type HttpHandler struct {
	appConfig     config.AppConfig
	server        *http.Server
	health        health.Registry
	security      security.SecurityManager
	cache         cache.Cache
	objectStorage objectstorage.Storage
	authHandler   authHandler.IAuthHandler
	userHandler   userHandler.IUserHandler
}

func NewHttpHandler(deps HttpHandlerDeps) IHttpHandler {
	return &HttpHandler{
		appConfig:     deps.AppConfig,
		server:        deps.Server,
		health:        deps.Health,
		security:      deps.Security,
		cache:         deps.Cache,
		objectStorage: deps.ObjectStorage,
		authHandler:   deps.AuthHandler,
		userHandler:   deps.UserHandler,
	}
}

func StartHttp(h IHttpHandler) {
	h.SetupRouter()
}

func (h *HttpHandler) SetupRouter() {

	// Register some custom health checks
	h.health.Register("example_check", &exampleHealthCheck{})

	// Wire security manager using declarative config from env.yaml (server.http.security)
	h.server.SetSecurityManager(h.security)

	r := h.server.Router()

	sigMiddleware := createSignatureMiddleware(h.appConfig.Server.HttpServer.Signature)

	r.GET("/auth/certs", h.authHandler.Certificates)

	v1 := r.Group("/v1")
	auth := v1.Group("/auth")
	{
		auth.POST("/password",
			http.Idempotency(h.cache,
				// Custom canonical key: prioritaskan header X-Idempotency-Key jika ada,
				// atau kombinasikan dengan username dari request body,
				// atau fallback ke DefaultCanonicalKey.
				http.WithCanonicalKeyFunc(func(c *gin.Context, body []byte) (string, error) {
					if key := c.GetHeader("X-Idempotency-Key"); key != "" {
						return "auth:password:" + key, nil
					}

					var req struct {
						Username string `json:"username"`
					}
					if err := json.Unmarshal(body, &req); err == nil && req.Username != "" {
						return "auth:password:" + req.Username, nil
					}

					return http.DefaultCanonicalKey(c, body)
				}),
				http.WithIdempotencyTTL(1*time.Minute),
			),
			h.authHandler.Password,
		)
		auth.POST("/logout",
			h.server.SecurityMiddleware(),
			h.authHandler.Logout,
		)
		auth.POST("/refresh",
			h.authHandler.TokenRefresh,
		)
	}

	user := v1.Group("/user")
	{
		user.GET("/detail",
			h.server.SecurityMiddleware(),
			h.server.RequirePermission("user:read"),
			h.userHandler.UserDetail,
		)
		user.GET("/update",
			sigMiddleware,
			h.server.SecurityMiddleware(),
			h.server.RequirePermission("user:update"),
			h.userHandler.Update,
		)
	}

	storageGroup := v1.Group("/storage")
	{
		// Upload file: POST /example/v1/storage/upload (multipart form with 'file' field)
		storageGroup.POST("/upload", func(c *gin.Context) {
			if h.objectStorage == nil {
				c.JSON(500, gin.H{"error": "object storage is not configured"})
				return
			}
			file, header, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(400, gin.H{"error": fmt.Sprintf("failed to get file from form: %v", err)})
				return
			}
			defer file.Close()

			objectName := header.Filename
			if customName := c.Request.FormValue("name"); customName != "" {
				objectName = customName
			}

			contentType := header.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			err = h.objectStorage.Upload(c.Request.Context(), objectName, file, header.Size, contentType)
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
				return
			}

			presignedURL, _ := h.objectStorage.PresignedURL(c.Request.Context(), objectName, 15*time.Minute)

			c.JSON(200, gin.H{
				"message":       "file uploaded successfully",
				"object_name":   objectName,
				"size":          header.Size,
				"content_type":  contentType,
				"presigned_url": presignedURL,
			})
		})

		// Download file: GET /example/v1/storage/download/:filename
		storageGroup.GET("/download/:filename", func(c *gin.Context) {
			if h.objectStorage == nil {
				c.JSON(500, gin.H{"error": "object storage is not configured"})
				return
			}
			filename := c.Param("filename")
			if filename == "" {
				c.JSON(400, gin.H{"error": "filename parameter is required"})
				return
			}

			rc, err := h.objectStorage.Download(c.Request.Context(), filename)
			if err != nil {
				c.JSON(404, gin.H{"error": fmt.Sprintf("object not found: %v", err)})
				return
			}
			defer rc.Close()

			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
			c.Header("Content-Type", "application/octet-stream")
			_, _ = io.Copy(c.Writer, rc)
		})

		// Presigned URL: GET /example/v1/storage/url/:filename
		storageGroup.GET("/url/:filename", func(c *gin.Context) {
			if h.objectStorage == nil {
				c.JSON(500, gin.H{"error": "object storage is not configured"})
				return
			}
			filename := c.Param("filename")
			url, err := h.objectStorage.PresignedURL(c.Request.Context(), filename, 15*time.Minute)
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("failed to generate url: %v", err)})
				return
			}
			c.JSON(200, gin.H{
				"object_name":   filename,
				"presigned_url": url,
				"expiry":        "15m",
			})
		})
	}
}

func createSignatureMiddleware(cfg http.SignatureConfig) gin.HandlerFunc {
	secret := []byte(cfg.Secret)
	if len(secret) == 0 {
		secret = []byte("example-secret-key-12345")
	}

	opts := []http.SignatureOption{
		http.WithSignatureResponseSigning(cfg.SignResponse),
	}
	if cfg.MaxClockDrift > 0 {
		opts = append(opts, http.WithSignatureMaxClockDrift(cfg.MaxClockDrift))
	}

	return http.Signature(http.StaticSecret(secret), opts...)
}

type exampleHealthCheck struct{}

func (e *exampleHealthCheck) Check(ctx context.Context) health.Health {
	return health.Up()
}
