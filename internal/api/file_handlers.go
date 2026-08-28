package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	goauth "github.com/Ojas1804/OAADrive/internal/auth"
	"github.com/Ojas1804/OAADrive/internal/db"
	"github.com/Ojas1804/OAADrive/internal/storage"
)

// maxUploadSize is a safety cap for the proxy-upload endpoint. The real
// limit enforced per-request is the user's remaining storage quota.
const maxUploadSize = 2 << 30 // 2 GiB

const presignTTL = 15 * time.Minute

type fileResponse struct {
	ID               int64  `json:"id"`
	OwnerID          int64  `json:"owner_id"`
	OriginalFilename string `json:"original_filename"`
	SizeBytes        int64  `json:"size_bytes"`
	MimeType         string `json:"mime_type,omitempty"`
	ChecksumSHA256   string `json:"checksum_sha256,omitempty"`
	CreatedAt        string `json:"created_at"`
}

func toFileResponse(f *db.File) fileResponse {
	resp := fileResponse{
		ID:               f.ID,
		OwnerID:          f.OwnerID,
		OriginalFilename: f.OriginalFilename,
		SizeBytes:        f.SizeBytes,
		CreatedAt:        f.CreatedAt.UTC().Format(time.RFC3339),
	}
	if f.MimeType != nil {
		resp.MimeType = *f.MimeType
	}
	if f.ChecksumSHA256 != nil {
		resp.ChecksumSHA256 = *f.ChecksumSHA256
	}
	return resp
}

// POST /api/v1/files/upload
func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	user, _ := goauth.UserFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	owner, err := s.Users.GetByID(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load user")
		return
	}

	used, err := s.Files.SumSizeByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not check quota")
		return
	}
	if used+header.Size > owner.StorageQuotaBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "storage quota exceeded")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	hasher := sha256.New()
	bucket, key := storage.Placement(user.ID, mimeType, header.Filename)

	if err := s.Storage.Upload(r.Context(), bucket, key, io.TeeReader(file, hasher), header.Size, mimeType); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store file")
		return
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	created, err := s.Files.Create(r.Context(), &db.File{
		OwnerID:          user.ID,
		OriginalFilename: header.Filename,
		StorageBucket:    bucket,
		StorageObjectKey: key,
		SizeBytes:        header.Size,
		MimeType:         &mimeType,
		ChecksumSHA256:   &checksum,
	})
	if err != nil {
		_ = s.Storage.Delete(r.Context(), bucket, key)
		writeError(w, http.StatusInternalServerError, "failed to save file metadata")
		return
	}

	_ = s.Audit.Log(r.Context(), &user.ID, "file_upload", created.OriginalFilename)

	writeJSON(w, http.StatusCreated, toFileResponse(created))
}

type presignUploadRequest struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	MimeType  string `json:"mime_type"`
}

// POST /api/v1/files/presign-upload
func (s *Server) handlePresignUpload(w http.ResponseWriter, r *http.Request) {
	user, _ := goauth.UserFromContext(r.Context())

	var req presignUploadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	owner, err := s.Users.GetByID(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load user")
		return
	}
	used, err := s.Files.SumSizeByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not check quota")
		return
	}
	if used+req.SizeBytes > owner.StorageQuotaBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "storage quota exceeded")
		return
	}

	bucket, key := storage.Placement(user.ID, req.MimeType, req.Filename)
	url, err := s.Storage.PresignPut(r.Context(), bucket, key, presignTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create upload URL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_url": url,
		"object_key": key,
		"expires_in": int(presignTTL.Seconds()),
	})
}

// GET /api/v1/files
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	user, _ := goauth.UserFromContext(r.Context())

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	pageSize := 50
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 200 {
			pageSize = v
		}
	}
	mimePrefix := r.URL.Query().Get("mime_prefix")

	files, total, err := s.Files.ListByOwner(r.Context(), user.ID, mimePrefix, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}

	resp := make([]fileResponse, 0, len(files))
	for _, f := range files {
		resp = append(resp, toFileResponse(f))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files": resp,
		"page":  page,
		"total": total,
	})
}

// GET /api/v1/files/{id}/download
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	user, _ := goauth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	file, err := s.Files.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if file.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "not the owner")
		return
	}

	url, err := s.Storage.PresignGet(r.Context(), file.StorageBucket, file.StorageObjectKey, presignTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create download URL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"download_url": url,
		"expires_in":   int(presignTTL.Seconds()),
	})
}

// DELETE /api/v1/files/{id}
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	user, _ := goauth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	file, err := s.Files.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if file.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "not the owner")
		return
	}

	if err := s.Files.SoftDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}
	_ = s.Storage.Delete(r.Context(), file.StorageBucket, file.StorageObjectKey)
	_ = s.Audit.Log(r.Context(), &user.ID, "file_delete", file.OriginalFilename)

	w.WriteHeader(http.StatusNoContent)
}
