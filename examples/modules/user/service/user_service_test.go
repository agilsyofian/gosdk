package service

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/user/domain"
	"github.com/agilsyofian/gosdk/security"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockUserRepository struct {
	lastUsername string
	lastDeviceID string
	user         *domain.User
	err          error
}

func (m *mockUserRepository) GetUser(ctx context.Context, subject, deviceID string) (*domain.User, error) {
	m.lastUsername = subject
	m.lastDeviceID = deviceID
	return m.user, m.err
}

func generateTestLogger() logger.Logger {
	return logger.NewZerologLogger(logger.Config{
		Output:              "stdout",
		LogLevel:            0,
		MaxLengthParamValue: 256,
	})
}

func TestUserService_GetUser_Success(t *testing.T) {
	expectedID := uuid.New()
	mockRepo := &mockUserRepository{
		user: &domain.User{
			UserId:   expectedID,
			Subject:  "test_user",
			FullName: "Test User",
			Email:    "test@example.com",
		},
	}

	svc := New(UserServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		RepoUser:  mockRepo,
	})

	res, err := svc.GetUser(context.Background(), "test_user", "dev-123")
	assert.NoError(t, err)
	assert.Equal(t, expectedID, res.UserId)
	assert.Equal(t, "test_user", res.Subject)
	assert.Equal(t, "test_user", mockRepo.lastUsername)
	assert.Equal(t, "dev-123", mockRepo.lastDeviceID)
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	mockRepo := &mockUserRepository{
		err: errors.NotFound(errors.String("user not found"), "user", "4004", "user not found"),
	}

	svc := New(UserServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		RepoUser:  mockRepo,
	})

	_, err := svc.GetUser(context.Background(), "unknown", "dev-123")
	assert.Error(t, err)
}

func TestUserService_Detail_NoSession(t *testing.T) {
	svc := New(UserServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		RepoUser:  &mockUserRepository{},
	})

	// Calling Detail with empty context (no security context) should return Unauthorized
	_, err := svc.Detail(context.Background())
	assert.Error(t, err)
}

func TestUserService_Detail_WithSession(t *testing.T) {
	userID := uuid.New()
	principal := &security.Principal{
		Subject:  userID.String(),
		Username: "session_user",
		Roles:    []string{"admin"},
		Claims: map[string]interface{}{
			"fullName": "Session User",
			"email":    "session@example.com",
		},
	}

	secCtx := &security.SecurityContext{
		Authenticated: true,
		Session: &security.Session{
			ID:        "sess-123",
			Principal: principal,
		},
	}
	ctx := security.WithContext(context.Background(), secCtx)

	svc := New(UserServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		RepoUser:  &mockUserRepository{},
	})

	res, err := svc.Detail(ctx)
	assert.NoError(t, err)
	assert.Equal(t, userID, res.UserId)
	assert.Equal(t, "session_user", res.Subject)
	assert.Equal(t, "Session User", res.FullName)
	assert.Equal(t, "session@example.com", res.Email)
}
