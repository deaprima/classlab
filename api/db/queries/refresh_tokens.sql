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

-- Expired tokens can no longer be used, so keeping them only grows the table.
-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < $1;

-- After a password change every session except the one making the change ends. With no matching
-- token_hash the subquery is NULL and every session is revoked.
-- name: RevokeUserTokensExceptFamilyOf :exec
UPDATE refresh_tokens SET revoked_at = sqlc.arg(revoked_at)
WHERE refresh_tokens.user_id = sqlc.arg(user_id) AND refresh_tokens.revoked_at IS NULL
  AND refresh_tokens.family_id IS DISTINCT FROM (
    SELECT t.family_id FROM refresh_tokens t
    WHERE t.token_hash = sqlc.arg(token_hash) AND t.user_id = sqlc.arg(user_id)
  );
