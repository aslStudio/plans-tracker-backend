package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"plans-tracker-backend/internal/accounts"
)

type Repository struct {
	Accounts *pgxpool.Pool
	Tasks    *pgxpool.Pool
	Habits   *pgxpool.Pool
	Calendar *pgxpool.Pool
}

func (r Repository) SaveChallenge(ctx context.Context, hash []byte, expiry time.Time) error {
	if _, err := r.Accounts.Exec(ctx, `DELETE FROM auth_challenges WHERE expires_at < now()`); err != nil {
		return err
	}
	_, err := r.Accounts.Exec(ctx, `INSERT INTO auth_challenges (nonce_hash, expires_at) VALUES ($1,$2)`, hash, expiry)
	return err
}
func (r Repository) ConsumeChallenge(ctx context.Context, hash []byte) (bool, error) {
	var found int
	err := r.Accounts.QueryRow(ctx, `DELETE FROM auth_challenges WHERE nonce_hash=$1 AND expires_at>now() RETURNING 1`, hash).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func (r Repository) UpsertAppleIdentity(ctx context.Context, identity accounts.AppleIdentity, displayName *string, encryptedRefresh []byte) (accounts.User, error) {
	tx, err := r.Accounts.Begin(ctx)
	if err != nil {
		return accounts.User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identity.Subject); err != nil {
		return accounts.User{}, err
	}
	var user accounts.User
	err = tx.QueryRow(ctx, `SELECT u.id,u.display_name FROM apple_identities AS a JOIN users AS u ON u.id=a.user_id WHERE a.apple_sub=$1`, identity.Subject).Scan(&user.ID, &user.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO users (display_name) VALUES ($1) RETURNING id,display_name`, displayName).Scan(&user.ID, &user.DisplayName)
		if err != nil {
			return accounts.User{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO apple_identities (apple_sub,user_id,email,refresh_token_ciphertext,last_verified_at) VALUES ($1,$2,$3,$4,now())`, identity.Subject, user.ID, identity.Email, encryptedRefresh)
		if err != nil {
			return accounts.User{}, err
		}
	} else if err != nil {
		return accounts.User{}, err
	} else {
		_, err = tx.Exec(ctx, `UPDATE apple_identities SET email=COALESCE($2,email),refresh_token_ciphertext=$3,last_verified_at=now(),updated_at=now() WHERE apple_sub=$1`, identity.Subject, identity.Email, encryptedRefresh)
		if err != nil {
			return accounts.User{}, err
		}
		if displayName != nil {
			_, err = tx.Exec(ctx, `UPDATE users SET display_name=COALESCE(display_name,$2),updated_at=now() WHERE id=$1`, user.ID, displayName)
			if err != nil {
				return accounts.User{}, err
			}
		}
	}
	err = tx.QueryRow(ctx, `SELECT a.email,u.display_name FROM apple_identities AS a JOIN users AS u ON u.id=a.user_id WHERE a.apple_sub=$1`, identity.Subject).Scan(&user.Email, &user.DisplayName)
	if err != nil {
		return accounts.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return accounts.User{}, err
	}
	return user, nil
}
func (r Repository) NewSession(ctx context.Context, user string, access, refresh []byte, accessExpiry, refreshExpiry time.Time) error {
	_, err := r.Accounts.Exec(ctx, `INSERT INTO auth_sessions (user_id,access_hash,refresh_hash,access_expires_at,refresh_expires_at) VALUES ($1,$2,$3,$4,$5)`, user, access, refresh, accessExpiry, refreshExpiry)
	return err
}
func (r Repository) LookupRefresh(ctx context.Context, hash []byte) (accounts.IdentityRecord, error) {
	var record accounts.IdentityRecord
	err := r.Accounts.QueryRow(ctx, `SELECT u.id,a.email,u.display_name,a.apple_sub,a.refresh_token_ciphertext,a.last_verified_at,u.deletion_pending
		FROM auth_sessions AS s JOIN users AS u ON u.id=s.user_id JOIN apple_identities AS a ON a.user_id=u.id
		WHERE s.refresh_hash=$1 AND s.refresh_expires_at>now() AND s.revoked_at IS NULL`, hash).Scan(
		&record.User.ID, &record.User.Email, &record.User.DisplayName, &record.AppleSub, &record.EncryptedRefresh, &record.LastVerifiedAt, &record.DeletionPending)
	if errors.Is(err, pgx.ErrNoRows) {
		return record, accounts.ErrUnauthorized
	}
	return record, err
}
func (r Repository) RotateSession(ctx context.Context, old, access, refresh []byte, accessExpiry, refreshExpiry time.Time) (bool, error) {
	result, err := r.Accounts.Exec(ctx, `UPDATE auth_sessions SET access_hash=$2,refresh_hash=$3,access_expires_at=$4,refresh_expires_at=$5,updated_at=now()
		WHERE refresh_hash=$1 AND refresh_expires_at>now() AND revoked_at IS NULL`, old, access, refresh, accessExpiry, refreshExpiry)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
func (r Repository) TouchAppleVerification(ctx context.Context, subject string) error {
	_, err := r.Accounts.Exec(ctx, `UPDATE apple_identities SET last_verified_at=now() WHERE apple_sub=$1`, subject)
	return err
}
func (r Repository) RevokeUserSessions(ctx context.Context, user string) error {
	_, err := r.Accounts.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now(),updated_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user)
	return err
}
func (r Repository) UserByAccess(ctx context.Context, hash []byte, allowDeleting bool) (accounts.User, error) {
	var user accounts.User
	err := r.Accounts.QueryRow(ctx, `SELECT u.id,a.email,u.display_name FROM auth_sessions AS s
		JOIN users AS u ON u.id=s.user_id LEFT JOIN apple_identities AS a ON a.user_id=u.id
		WHERE s.access_hash=$1 AND s.access_expires_at>now() AND s.revoked_at IS NULL AND (NOT u.deletion_pending OR $2)`, hash, allowDeleting).Scan(&user.ID, &user.Email, &user.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, accounts.ErrUnauthorized
	}
	return user, err
}
func (r Repository) RevokeSession(ctx context.Context, hash []byte) (bool, error) {
	result, err := r.Accounts.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now(),updated_at=now() WHERE access_hash=$1 AND access_expires_at>now() AND revoked_at IS NULL`, hash)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
func (r Repository) MarkDeleting(ctx context.Context, user string) error {
	_, err := r.Accounts.Exec(ctx, `UPDATE users SET deletion_pending=true,updated_at=now() WHERE id=$1`, user)
	return err
}
func (r Repository) AppleRefreshForUser(ctx context.Context, user string) ([]byte, error) {
	var encrypted []byte
	err := r.Accounts.QueryRow(ctx, `SELECT refresh_token_ciphertext FROM apple_identities WHERE user_id=$1`, user).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return encrypted, err
}

func (r Repository) NotificationSeen(ctx context.Context, id string) (bool, error) {
	var found bool
	err := r.Accounts.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM apple_notification_receipts WHERE jti=$1)`, id).Scan(&found)
	return found, err
}
func (r Repository) UserIDByAppleSub(ctx context.Context, sub string) (string, error) {
	var id string
	err := r.Accounts.QueryRow(ctx, `SELECT user_id FROM apple_identities WHERE apple_sub=$1`, sub).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}
func (r Repository) SetEmailForwarding(ctx context.Context, sub string, enabled bool, email *string) error {
	_, err := r.Accounts.Exec(ctx, `UPDATE apple_identities SET email_forwarding_enabled=$2,email=COALESCE($3,email),updated_at=now() WHERE apple_sub=$1`, sub, enabled, email)
	return err
}
func (r Repository) ClearAppleRefresh(ctx context.Context, sub string) error {
	_, err := r.Accounts.Exec(ctx, `UPDATE apple_identities SET refresh_token_ciphertext=NULL,last_verified_at=NULL,updated_at=now() WHERE apple_sub=$1`, sub)
	return err
}
func (r Repository) RecordNotification(ctx context.Context, id string) error {
	_, err := r.Accounts.Exec(ctx, `INSERT INTO apple_notification_receipts (jti) VALUES ($1) ON CONFLICT DO NOTHING`, id)
	return err
}
func (r Repository) DeleteUser(ctx context.Context, user string) error {
	_, err := r.Accounts.Exec(ctx, `DELETE FROM users WHERE id=$1 AND deletion_pending`, user)
	return err
}

func (r Repository) DeleteUserData(ctx context.Context, user string) error {
	if r.Tasks == nil || r.Habits == nil || r.Calendar == nil {
		return accounts.ErrDeletionUnavailable
	}
	if err := deleteInDatabase(ctx, r.Tasks, user, []string{
		`DELETE FROM tasks WHERE user_id=$1`,
		`DELETE FROM task_statuses WHERE user_id=$1`,
		`DELETE FROM task_priorities WHERE user_id=$1`,
		`DELETE FROM task_groups WHERE user_id=$1`,
		`DELETE FROM calendar_events WHERE user_id=$1`,
	}); err != nil {
		return err
	}
	if err := deleteInDatabase(ctx, r.Habits, user, []string{
		`DELETE FROM habits WHERE user_id=$1`,
		`DELETE FROM habit_categories WHERE user_id=$1`,
	}); err != nil {
		return err
	}
	return deleteInDatabase(ctx, r.Calendar, user, []string{
		`DELETE FROM calendar_events WHERE user_id=$1`,
		`DELETE FROM calendar_settings WHERE user_id=$1`,
	})
}

func deleteInDatabase(ctx context.Context, pool *pgxpool.Pool, user string, statements []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement, user); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
