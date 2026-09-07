package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/auth/model"
	pbUser "github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/jwks"
	crypto "github.com/agilsyofian/gosdk/utils/crypto"
)

// mockUserIntegration implements user.IClientUser for testing.
type mockUserIntegration struct {
	lastUsername string
	lastDeviceID string
	response     *pbUser.Response
	err          error
}

func (m *mockUserIntegration) GetUser(ctx context.Context, username, deviceID string) (*pbUser.Response, error) {
	m.lastUsername = username
	m.lastDeviceID = deviceID
	return m.response, m.err
}

// mockPublisher implements IUserEventPublisher for testing.
type mockPublisher struct{}

func (m *mockPublisher) PublishUserEvent(ctx context.Context, event UserEvent) error {
	return nil
}

// mockSessionManager implements security.SessionManager for testing.
type mockSessionManager struct{}

func (m *mockSessionManager) Create(ctx context.Context, sessionID string, principal *security.Principal, ttl time.Duration) (*security.Session, error) {
	return &security.Session{ID: sessionID, Principal: principal}, nil
}
func (m *mockSessionManager) Get(ctx context.Context, sessionID string) (*security.Session, error) {
	return nil, nil
}
func (m *mockSessionManager) Delete(ctx context.Context, sessionID string) error {
	return nil
}
func (m *mockSessionManager) Refresh(ctx context.Context, sessionID string, ttl time.Duration) (*security.Session, error) {
	return nil, nil
}

func setupTestKeys(t *testing.T) (*rsa.PrivateKey, *crypto.FieldDecryptor, *crypto.FieldEncryptor) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	jwk := crypto.PublicKeyToJWK(&priv.PublicKey, "enc_v1")
	kp, err := crypto.NewJWKSKeyProvider(crypto.Jwks{Keys: []crypto.Key{jwk}})
	if err != nil {
		t.Fatalf("NewJWKSKeyProvider: %v", err)
	}
	if err := kp.SetPrivateKey("enc_v1", priv); err != nil {
		t.Fatalf("SetPrivateKey: %v", err)
	}
	if err := kp.SetActive("enc_v1"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	dec := crypto.NewFieldDecryptor(kp)
	enc := crypto.NewFieldEncryptor(kp)
	return priv, dec, enc
}

func generateTestLogger() logger.Logger {
	return logger.NewZerologLogger(logger.Config{
		Output:              "stdout",
		LogLevel:            0,
		MaxLengthParamValue: 256,
	})
}

func TestAuthService_LoginPassword_WithEncryptedFields(t *testing.T) {
	_, decryptor, encryptor := setupTestKeys(t)

	// Encrypt username and password using FieldEncryptor
	ctx := context.Background()
	reqBody := map[string]any{
		"username": "john_doe",
		"password": "secret_password_123",
	}
	encReq, err := encryptor.Encrypt(ctx, "enc_v1", reqBody, []string{"username", "password"})
	if err != nil {
		t.Fatalf("FieldEncryptor.Encrypt: %v", err)
	}

	var encUsername, encPassword string
	if err := json.Unmarshal(encReq.Body["username"], &encUsername); err != nil {
		t.Fatalf("unmarshal enc username: %v", err)
	}
	if err := json.Unmarshal(encReq.Body["password"], &encPassword); err != nil {
		t.Fatalf("unmarshal enc password: %v", err)
	}

	// Prepare metadata from encrypted request headers
	md := metadata.New()
	md.Set("x-device-id", "device-xyz-123")
	md.Set("x-encryption-version", encReq.Headers.Version)
	md.Set("x-encryption-kid", encReq.Headers.KeyID)
	md.Set("x-encryption-key", encReq.Headers.EncryptedKey)
	md.Set("x-nonce", encReq.Headers.Nonce)
	md.Set("x-timestamp", encReq.Headers.Timestamp)
	ctx = metadata.WithContext(ctx, md)

	userMock := &mockUserIntegration{
		response: &pbUser.Response{
			UserId:            "user-1",
			Subject:           "john_doe",
			DevicesIdentifier: "device-xyz-123",
		},
	}

	appCfg := config.AppConfig{
		AppName: "test-app",
	}
	appCfg.Security.Kid.Signing = "sig_v1"

	svc := &AuthService{
		appConfig:       appCfg,
		logger:          generateTestLogger(),
		userIntegration: userMock,
		publisher:       &mockPublisher{},
		session:         &mockSessionManager{},
		fieldDecryptor:  decryptor,
		jwks:            jwks.Jwks{},
	}

	loginCmd := model.Login{
		Username: encUsername,
		Password: encPassword,
	}

	// LoginPassword will decrypt and then call userIntegration with plaintext username
	_, _ = svc.LoginPassword(ctx, loginCmd)

	if userMock.lastUsername != "john_doe" {
		t.Errorf("expected decrypted username 'john_doe', got %q", userMock.lastUsername)
	}
	if userMock.lastDeviceID != "device-xyz-123" {
		t.Errorf("expected device id 'device-xyz-123', got %q", userMock.lastDeviceID)
	}
}

func TestAuthService_LoginPassword_DecryptionFailsOnBadHeader(t *testing.T) {
	_, decryptor, _ := setupTestKeys(t)

	ctx := context.Background()
	md := metadata.New()
	md.Set("x-device-id", "device-123")
	md.Set("x-encryption-version", "1")
	md.Set("x-encryption-kid", "unknown_kid")
	md.Set("x-encryption-key", "invalid_key")
	md.Set("x-nonce", "invalid_nonce")
	md.Set("x-timestamp", "12345")
	ctx = metadata.WithContext(ctx, md)

	svc := &AuthService{
		appConfig:      config.AppConfig{AppName: "test-app"},
		logger:         generateTestLogger(),
		fieldDecryptor: decryptor,
	}

	_, err := svc.LoginPassword(ctx, model.Login{
		Username: "some_data",
		Password: "some_password",
	})

	if err == nil {
		t.Fatal("expected decryption error, got nil")
	}

	var appErr *errors.AppError
	if stderrors.As(err, &appErr) {
		if appErr.Code != codeValidationFailed {
			t.Errorf("expected error code %s, got %s", codeValidationFailed, appErr.Code)
		}
	}
}
