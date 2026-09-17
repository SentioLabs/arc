package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
)

// planRequestIdentity scopes a retained key to one project and operation target.
// Fingerprint covers canonical request fields, including exact content bytes.
type planRequestIdentity struct {
	projectID, scope, key, fingerprint string
}

type planRequestReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// The JSON encoding of typed requests canonically includes the exact content
// string and all concurrency fields. Operation/target are part of the scope.
func planFingerprint(request any) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// lookupPlanRequest accepts either a connection or the caller write transaction.
// A payload mismatch is a conflict even when mutable plan state has moved on.
// The stored response is authoritative for replay, not a freshly read head.
func lookupPlanRequest(
	ctx context.Context,
	q planRequestReader,
	identity planRequestIdentity,
) (*storage.PlanWriteResult, bool, error) {
	var stored, body string
	err := q.QueryRowContext(
		ctx,
		"SELECT fingerprint,result FROM plan_idempotency WHERE project_id=? AND scope=? AND key=?",
		identity.projectID,
		identity.scope,
		identity.key,
	).
		Scan(
			&stored,
			&body,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if stored != identity.fingerprint {
		return nil, true, storage.ErrPlanConflict
	}
	var result storage.PlanWriteResult
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, true, err
	}
	result.Replay = true
	return &result, true, nil
}

// recordPlanRequest reserves only successful operations inside their artifact transaction.
// Keys have no expiration in this release and remain with retained history.
func recordPlanRequest(
	ctx context.Context,
	tx *sql.Tx,
	identity planRequestIdentity,
	result *storage.PlanWriteResult,
) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		"INSERT INTO plan_idempotency(project_id,scope,key,fingerprint,result,created_at) VALUES(?,?,?,?,?,?)",
		identity.projectID,
		identity.scope,
		identity.key,
		identity.fingerprint,
		string(body),
		time.Now().UTC(),
	)
	return err
}
