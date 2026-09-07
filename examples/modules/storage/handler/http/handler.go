package http

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/gin-gonic/gin"
)

type IStorageHandler interface {
	Upload(c *gin.Context)
	Download(c *gin.Context)
	PresignedURL(c *gin.Context)
}

type StorageHandler struct {
	store objectstorage.Storage
}

func NewHandler(store objectstorage.Storage) IStorageHandler {
	return &StorageHandler{
		store: store,
	}
}

func (h *StorageHandler) Upload(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "object storage is not configured"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to get file from form: %v", err)})
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

	err = h.store.Upload(c.Request.Context(), objectName, file, header.Size, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("upload failed: %v", err)})
		return
	}

	presignedURL, _ := h.store.PresignedURL(c.Request.Context(), objectName, 15*time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"message":       "file uploaded successfully",
		"object_name":   objectName,
		"size":          header.Size,
		"content_type":  contentType,
		"presigned_url": presignedURL,
	})
}

func (h *StorageHandler) Download(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "object storage is not configured"})
		return
	}
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename parameter is required"})
		return
	}

	rc, err := h.store.Download(c.Request.Context(), filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("object not found: %v", err)})
		return
	}
	defer rc.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/octet-stream")
	_, _ = io.Copy(c.Writer, rc)
}

func (h *StorageHandler) PresignedURL(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "object storage is not configured"})
		return
	}
	filename := c.Param("filename")
	url, err := h.store.PresignedURL(c.Request.Context(), filename, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to generate url: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"object_name":   filename,
		"presigned_url": url,
		"expiry":        "15m",
	})
}
