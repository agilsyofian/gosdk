package repository

import (
	"context"
	"database/sql"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/storage/database"
	"github.com/lib/pq"
	"user-service/modules/user/domain"
)

type IUserRepository interface {
	GetUser(ctx context.Context, subject, deviceId string) (*domain.User, error)
}

type userRepository struct {
	db database.Database
}

func (r userRepository) GetUser(ctx context.Context, subject, deviceId string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, status, email, phone_number, full_name, display_name,
		       avatar_url, locale, timezone, subject, password_hash,
		       biometric_enabled, biometric_hash, roles, permissions,
		       device_identifier, platform, device_name, push_token
		FROM v_users
		WHERE subject = $1 AND device_identifier = $2
		LIMIT 1
	`, subject, deviceId)

	var detail domain.User
	var roles, permissions pq.StringArray
	var phoneNumber, fullName, displayName, avatarURL, locale, timezone sql.NullString
	var authSubject, passwordHash, biometricHash sql.NullString
	var biometricEnabled sql.NullBool
	var deviceIdentifier, platform, deviceName, pushToken sql.NullString
	if err := row.Scan(
		&detail.UserId,
		&detail.Status,
		&detail.Email,
		&phoneNumber,
		&fullName,
		&displayName,
		&avatarURL,
		&locale,
		&timezone,
		&authSubject,
		&passwordHash,
		&biometricEnabled,
		&biometricHash,
		&roles,
		&permissions,
		&deviceIdentifier,
		&platform,
		&deviceName,
		&pushToken,
	); err != nil {
		return nil, errors.NotFound(err, "auth repository", "4040", "user detail not found")
	}

	detail.PhoneNumber = phoneNumber.String
	detail.FullName = fullName.String
	detail.DisplayName = displayName.String
	detail.AvatarUrl = avatarURL.String
	detail.Locale = locale.String
	detail.Timezone = timezone.String
	detail.Subject = authSubject.String
	detail.PasswordHash = passwordHash.String
	detail.BiometricEnabled = biometricEnabled.Valid && biometricEnabled.Bool
	detail.BiometricHash = biometricHash.String
	detail.DevicesIdentifier = deviceIdentifier.String
	detail.Platform = platform.String
	detail.DeviceName = deviceName.String
	detail.PushToken = pushToken.String
	detail.Roles = []string(roles)
	detail.Permissions = []string(permissions)
	return &detail, nil
}

func New(db database.Database) IUserRepository {
	return &userRepository{
		db: db,
	}
}
