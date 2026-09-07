package httphandler

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/example/modules/storage/domain"
	"github.com/agilsyofian/gosdk/example/modules/storage/service"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

type IStorageHandler interface {
	Upload(c *gin.Context)
	Download(c *gin.Context)
	PresignedURL(c *gin.Context)
}

const (
	ErrorFileNameRequired = "file name is required"
)

type StorageHandler struct {
	service service.IStorageService
}

func New(service service.IStorageService) IStorageHandler {
	return &StorageHandler{
		service: service,
	}
}

func (h *StorageHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		_ = c.Error(errors.BadRequest(err, "storage", "4001", fmt.Sprintf("failed to get file from form: %v", err)))
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

	res, err := h.service.Upload(c.Request.Context(), domain.UploadFileCmd{
		ObjectName:  objectName,
		Content:     file,
		Size:        header.Size,
		ContentType: contentType,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.New(c.Request.Context(), res).Success())
}

func (h *StorageHandler) Download(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		_ = c.Error(errors.BadRequest(errors.String(ErrorFileNameRequired), "storage", "4001", ErrorFileNameRequired))
		return
	}

	rc, err := h.service.Download(c.Request.Context(), filename)
	if err != nil {
		_ = c.Error(err)
		return
	}
	defer rc.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/octet-stream")
	_, _ = io.Copy(c.Writer, rc)
}

func (h *StorageHandler) PresignedURL(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		_ = c.Error(errors.BadRequest(errors.String(ErrorFileNameRequired), "storage", "4001", ErrorFileNameRequired))
		return
	}

	url, err := h.service.PresignedURL(c.Request.Context(), filename, 15*time.Minute)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.New(c.Request.Context(), gin.H{
		"objectName":   filename,
		"presignedUrl": url,
		"expiry":       "15m",
	}).Success())
}
