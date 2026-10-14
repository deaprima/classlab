-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5);

-- Row lock serialises concurrent refreshes of the same token so only one can rotate it.
-- name: GetRefreshTokenForUpdate :one
SELECT id, user_id, family_id, token_hash, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenFamily :exec
UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeFamilyOfTokenForUser :exec
UPDATE refresh_tokens SET revoked_at = sqlc.arg(revoked_at)
WHERE revoked_at IS NULL
  AND family_id = (
    SELECT t.family_id FROM refresh_tokens t
    WHERE t.token_hash = sqlc.arg(token_hash) AND t.user_id = sqlc.arg(user_id)
  );
