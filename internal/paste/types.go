// Package paste provides zero-knowledge encrypted paste storage for arc plans
// and review comments. The server stores opaque ciphertext blobs; encryption
// and decryption happen exclusively on clients.
package paste

import "time"

type PasteShare struct {
	ID        string     `json:"id"`
	PlanBlob  []byte     `json:"plan_blob"`
	PlanIV    []byte     `json:"plan_iv"`
	SchemaVer int        `json:"schema_ver"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type PasteEvent struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	Blob      []byte    `json:"blob"`
	IV        []byte    `json:"iv"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePasteRequest struct {
	PlanBlob  []byte     `json:"plan_blob"`
	PlanIV    []byte     `json:"plan_iv"`
	SchemaVer int        `json:"schema_ver"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type CreatePasteResponse struct {
	ID        string `json:"id"`
	EditToken string `json:"edit_token"`
}

type AppendEventRequest struct {
	Blob []byte `json:"blob"`
	IV   []byte `json:"iv"`
}

type GetPasteResponse struct {
	PasteShare
	Events []PasteEvent `json:"events"`
}
