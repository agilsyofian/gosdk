package domain

import "io"

// UploadFileCmd represents the command to upload an object to storage.
type UploadFileCmd struct {
	ObjectName  string
	Content     io.Reader
	Size        int64
	ContentType string
}

// UploadedFile represents the result of a successful upload.
type UploadedFile struct {
	ObjectName   string `json:"objectName"`
	PresignedURL string `json:"presignedUrl,omitempty"`
}

// FileInfo represents the metadata of a stored object.
type FileInfo struct {
	ObjectName  string `json:"objectName"`
	DownloadURL string `json:"downloadUrl"`
}
