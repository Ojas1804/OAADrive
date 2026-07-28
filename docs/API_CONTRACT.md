# OAADrive API Contract

Base URL: `http://<server>:8080/api/v1`

Auth: JWT in `HttpOnly` cookie `oaadrive_session`

Error envelope (all non-2xx responses):

```json
{ "error": "message" }
```

---

## Auth

### POST /auth/login
- **Auth:** none
- **Request:**
  ```json
  { "email": "string", "password": "string" }
  ```
- **Responses:**
  - `200 OK` — sets `oaadrive_session` cookie
    ```json
    { "user": { "id": 1, "name": "string", "email": "string", "role": "admin|member" } }
    ```
  - `401 Unauthorized` — invalid credentials

### POST /auth/logout
- **Auth:** user
- **Request:** none
- **Responses:** `204 No Content` — clears session cookie

---

## Files

### POST /files/upload
- **Auth:** user
- **Request:** `multipart/form-data` with a `file` field
- **Responses:**
  - `201 Created`
    ```json
    {
      "id": 1,
      "owner_id": 1,
      "original_filename": "string",
      "size_bytes": 12345,
      "mime_type": "image/jpeg",
      "checksum_sha256": "string",
      "created_at": "RFC3339"
    }
    ```
  - `413 Payload Too Large` — quota exceeded
  - `409 Conflict` — checksum mismatch

### POST /files/presign-upload
- **Auth:** user
- **Request:**
  ```json
  { "filename": "string", "size_bytes": 12345, "mime_type": "image/jpeg" }
  ```
- **Responses:**
  - `200 OK`
    ```json
    { "upload_url": "string", "object_key": "string", "expires_in": 900 }
    ```
  - `413 Payload Too Large` — quota exceeded

### GET /files?page=1&page_size=50&mime_prefix=image/
- **Auth:** user (returns only own files)
- **Responses:**
  - `200 OK`
    ```json
    {
      "files": [
        {
          "id": 1,
          "owner_id": 1,
          "original_filename": "string",
          "size_bytes": 12345,
          "mime_type": "image/jpeg",
          "checksum_sha256": "string",
          "created_at": "RFC3339"
        }
      ],
      "page": 1,
      "total": 42
    }
    ```

### GET /files/{id}/download
- **Auth:** owner
- **Responses:**
  - `200 OK`
    ```json
    { "download_url": "string", "expires_in": 900 }
    ```
  - `403 Forbidden` — not the owner
  - `404 Not Found`

### DELETE /files/{id}
- **Auth:** owner
- **Responses:**
  - `204 No Content`
  - `403 Forbidden` — not the owner
  - `404 Not Found`

---

## Admin

### GET /admin/users
- **Auth:** admin
- **Responses:**
  - `200 OK`
    ```json
    {
      "users": [
        {
          "id": 1,
          "name": "string",
          "email": "string",
          "role": "admin|member",
          "storage_quota_bytes": 5368709120,
          "created_at": "RFC3339"
        }
      ]
    }
    ```

### POST /admin/users
- **Auth:** admin
- **Request:**
  ```json
  {
    "name": "string",
    "email": "string",
    "password": "string",
    "role": "admin|member",
    "storage_quota_bytes": 5368709120
  }
  ```
- **Responses:**
  - `201 Created` — returns the created `User` object
  - `409 Conflict` — email already taken

### PATCH /admin/users/{id}/quota
- **Auth:** admin
- **Request:**
  ```json
  { "storage_quota_bytes": 10737418240 }
  ```
- **Responses:** `200 OK` — returns the updated `User` object

### DELETE /admin/users/{id}
- **Auth:** admin
- **Responses:** `204 No Content`

### GET /admin/storage-usage
- **Auth:** admin
- **Responses:**
  - `200 OK`
    ```json
    {
      "usage": [
        { "user_id": 1, "name": "string", "used_bytes": 12345, "quota_bytes": 5368709120 }
      ]
    }
    ```

---

## Schemas

### File
```json
{
  "id": 1,
  "owner_id": 1,
  "original_filename": "string",
  "size_bytes": 12345,
  "mime_type": "string",
  "checksum_sha256": "string",
  "created_at": "RFC3339"
}
```

### User
```json
{
  "id": 1,
  "name": "string",
  "email": "string",
  "role": "admin|member",
  "storage_quota_bytes": 5368709120,
  "created_at": "RFC3339"
}
```

---

## Contract Rules

- Users can only ever see and manage their own files (`owner_id` enforced server-side).
- All state-changing admin actions, logins, and file deletions are written to `audit_log`.
- Upload flow always enforces `SUM(size_bytes) + new_file <= storage_quota_bytes` before touching SeaweedFS.
- Postgres stores metadata only; file bytes live exclusively in SeaweedFS.
- Per-user object keys use prefixes inside shared buckets, e.g. `photos/<user_id>/<filename>`.
