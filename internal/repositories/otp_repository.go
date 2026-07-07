package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"nirvaan-backend/internal/models"
)

type OTPRepository struct {
	db *pgxpool.Pool
}

func NewOTPRepository(db *pgxpool.Pool) OTPRepository {
	return OTPRepository{db: db}
}

// DeleteOldOTPs removes any previous unused OTP records for this
// email + purpose combination before generating a new one.
func (r OTPRepository) DeleteOldOTPs(ctx context.Context, email, purpose string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM otp_codes WHERE email = $1 AND purpose = $2 AND used = FALSE`,
		email, purpose,
	)
	if err != nil {
		return fmt.Errorf("delete old otps: %w", err)
	}
	return nil
}

// Save inserts a new OTP record.
func (r OTPRepository) Save(ctx context.Context, otp *models.OTPCode) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO otp_codes (email, code, purpose, expires_at, used)
		 VALUES ($1, $2, $3, $4, FALSE)
		 RETURNING id, created_at, updated_at`,
		otp.Email,
		otp.Code,
		otp.Purpose,
		otp.ExpiresAt,
	).Scan(&otp.ID, &otp.CreatedAt, &otp.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save otp: %w", err)
	}
	return nil
}

    

// GetLatest retrieves the most recently created OTP for this email + purpose.
func (r OTPRepository) GetLatest(ctx context.Context, email, purpose string) (*models.OTPCode, error) {
	var otp models.OTPCode
	err := r.db.QueryRow(ctx,
		`SELECT id, email, code, purpose, expires_at, used, created_at, updated_at
		 FROM otp_codes
		 WHERE email = $1 AND purpose = $2
		 ORDER BY created_at DESC
		 LIMIT 1`,
		email, purpose,
	).Scan(
		&otp.ID,
		&otp.Email,
		&otp.Code,
		&otp.Purpose,
		&otp.ExpiresAt,
		&otp.Used,
		&otp.CreatedAt,
		&otp.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest otp: %w", err)
	}
	return &otp, nil
}

    
// MarkUsed marks the OTP with the given ID as consumed so it cannot be replayed.
func (r OTPRepository) MarkUsed(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE otp_codes SET used = TRUE, updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark otp used: %w", err)
	}
	return nil
}
