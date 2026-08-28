package storage

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Placement decides which bucket a file belongs in and generates a unique
// object key, per plan.md's "one bucket per purpose, per-user prefix" scheme.
func Placement(ownerID int64, mimeType, filename string) (bucket, key string) {
	bucket = BucketDocuments
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") {
		bucket = BucketPhotos
	}
	key = fmt.Sprintf("%d/%s-%s", ownerID, uuid.NewString(), filename)
	return bucket, key
}
