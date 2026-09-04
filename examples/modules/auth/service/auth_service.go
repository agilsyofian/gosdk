package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/integration/grpc/user"
	publisher2 "github.com/agilsyofian/gosdk/example/messaging/publisher"
	"github.com/agilsyofian/gosdk/example/modules/auth/model"
	pbUser "github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/security/jwks"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/transport/server/http/metadata"
	"github.com/agilsyofian/gosdk/utils/crypto"
	"github.com/google/uuid"
)

type IAuthService interface {
	Certificates(ctx context.Context) (res jwks.Jwks, err error)
	LoginPassword(ctx context.Context, cmd model.Login) (res model.Token, err error)
	Logout(ctx context.Context) (err error)
	TokenRefresh(ctx context.Context, token model.TokenRefresh) (res model.TokenRefresh, err error)
}

type AuthService struct {
	appConfig       config.AppConfig
	logger          logger.Logger
	cache           cache.Cache
	jwtConfig       jwt.Config
	jwks            jwks.Jwks
	session         security.SessionManager
	userIntegration user.IClientUser
	publisher       publisher2.IPublisher
	fieldDecryptor  *crypto.FieldDecryptor
}

func NewService(
	deps AuthServiceDeps,
) IAuthService {
	return &AuthService{
		appConfig:       deps.AppConfig,
		logger:          deps.Logger,
		cache:           deps.Cache,
		jwtConfig:       deps.JwtConfig,
		jwks:            deps.Jwks,
		session:         deps.Session,
		userIntegration: deps.UserIntegration,
		publisher:       deps.Publisher,
		fieldDecryptor:  deps.FieldDecryptor,
	}
}

func (s *AuthService) Certificates(ctx context.Context) (res jwks.Jwks, err error) {
	return s.jwks, nil
}

// Error codes for LoginPassword / Logout. Each failure branch gets its
// own code (see .instructions/api-guidelines.md) instead of reusing one
// code — or a stringified HTTP status — across unrelated causes.
const (
	codeMetadataMissing   = "4001"
	codeValidationFailed  = "4002"
	codeUserLookupFailed  = "4010"
	codeUserNotFound      = "4011"
	codeInvalidPassword   = "4012"
	codeSessionFailed     = "5001"
	codeEventMarshal      = "5002"
	codeEventPublish      = "5003"
	codeNoSecurityCtx     = "4013"
	codeDecryptionFailed  = "4020"
	codeDecryptOpenFailed = "4021"
)

func (s *AuthService) LoginPassword(ctx context.Context, cmd model.Login) (res model.Token, err error) {

	md, ok := metadata.FromContext(ctx)
	const message = "invalid login credentials"
	if !ok {
		return model.Token{}, errors.BadRequest(errors.String("metadata not found"), s.appConfig.AppName, codeMetadataMissing, message)
	}
	cmd.DeviceIdentifier = md.DeviceID

	// Decrypt Username and Password fields that were encrypted by the client
	// using the field-level encryption protocol (STEERFILE §14). The encryption
	// headers (X-Encryption-Version, X-Encryption-Kid, X-Encryption-Key,
	// X-Nonce, X-Timestamp) are extracted from the request metadata. If the
	// client did not encrypt the fields, FieldSession.Decrypt returns the value
	// unchanged (empty → empty, plaintext → plaintext is not valid base64url and
	// will error — enforce that by always requiring encrypted fields on this endpoint).
	cmd, err = s.decryptLoginFields(ctx, cmd, md)
	if err != nil {
		return model.Token{}, errors.BadRequest(err, s.appConfig.AppName, codeValidationFailed, message)
	}

	if err = ValidateAuthCommand(cmd); err != nil {
		return model.Token{}, errors.BadRequest(err, s.appConfig.AppName, codeValidationFailed, message)
	}

	userData, err := s.userIntegration.GetUser(ctx, cmd.Username, cmd.DeviceIdentifier)
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, codeUserLookupFailed, message)
	}
	if userData == nil {
		return model.Token{}, errors.Unauthorized(errors.String("user lookup returned no data"), s.appConfig.AppName, codeUserNotFound, message)
	}

	//if err := bcrypt.CompareHashAndPassword([]byte(userData.PasswordHash), []byte(cmd.Password)); err != nil {
	//	return model.Token{}, errors.Unauthorized(err, s.appConfig.AppName, codeInvalidPassword, message)
	//}

	res, err = s.generateJwtToken(ctx, userData)
	if err != nil {
		return model.Token{}, errors.Unauthorized(err, s.appConfig.AppName, codeSessionFailed, "failed to start session")
	}

	eventId := uuid.New().String()
	jsonRaw, err := json.Marshal(userData)
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, codeEventMarshal, "error marshaling user data")
	}
	err = s.publisher.UserEvent(ctx, publisher2.UserEvent{
		EventID:   eventId,
		EventType: publisher2.UserEventCreated,
		UserID:    userData.UserId,
		Data:      jsonRaw,
	})
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, codeEventPublish, "error publishing user event")
	}

	return res, err

}

// decryptLoginFields opens a FieldDecryptor session using the encryption
// headers present in request metadata, then decrypts cmd.Username and
// cmd.Password in-place. The decryption headers are expected to arrive as
// standard HTTP request headers and are mapped to FieldHeaders by this helper.
//
// The caller is responsible for running ValidateAuthCommand AFTER this returns
// so that the validator always sees the plaintext values.
func (s *AuthService) decryptLoginFields(ctx context.Context, cmd model.Login, md metadata.Request) (model.Login, error) {
	const message = "invalid login credentials"

	// Build FieldHeaders from the request metadata headers already extracted
	// by the HTTP middleware (transport/server/http/middleware/metadata.go).
	h := crypto.FieldHeaders{
		Version:      md.EncryptionVersion,
		KeyID:        md.EncryptionKid,
		EncryptedKey: md.EncryptionKey,
		Nonce:        md.Nonce,
		Timestamp:    md.Timestamp,
	}

	if s.fieldDecryptor == nil {
		s.logger.Error(ctx, "login: field decryptor is not configured", logger.LogPayload{
			Error: "field decryptor is nil",
		})
		return model.Login{}, errors.InternalServerError(
			errors.String("field decryptor not configured"),
			s.appConfig.AppName,
			codeDecryptOpenFailed,
			message,
		)
	}

	// Open validates and unwraps the per-request AES key using the configured
	// private encryption key. This is the RSA-OAEP step — done once per request.
	session, err := s.fieldDecryptor.Open(ctx, h)
	const errorMessage = "field decryption failed"
	if err != nil {
		s.logger.Warn(ctx, "login: field decryption open failed", logger.LogPayload{
			Error: err.Error(),
		})
		return model.Login{}, errors.BadRequest(
			errors.String(errorMessage),
			s.appConfig.AppName,
			codeDecryptOpenFailed,
			message,
		)
	}

	// Decrypt Username.
	username, err := session.Decrypt(cmd.Username)
	if err != nil {
		s.logger.Warn(ctx, "login: failed to decrypt username", logger.LogPayload{
			Error: err.Error(),
		})
		return model.Login{}, errors.BadRequest(
			errors.String(errorMessage),
			s.appConfig.AppName,
			codeDecryptionFailed,
			message,
		)
	}

	// Decrypt Password.
	password, err := session.Decrypt(cmd.Password)
	if err != nil {
		s.logger.Warn(ctx, "login: failed to decrypt password", logger.LogPayload{
			Error: err.Error(),
		})
		return model.Login{}, errors.BadRequest(
			errors.String(errorMessage),
			s.appConfig.AppName,
			codeDecryptionFailed,
			message,
		)
	}

	cmd.Username = username
	cmd.Password = password
	return cmd, nil
}

func (s *AuthService) Logout(ctx context.Context) (err error) {
	sessionUser, err := security.MustFromContext(ctx)
	if err != nil {
		return errors.Unauthorized(err, s.appConfig.AppName, codeNoSecurityCtx, "no active session")
	}
	return s.session.Delete(ctx, sessionUser.Principal.Subject)
}

// TokenRefresh is intentionally not implemented in this example. A real
// implementation needs to: verify the incoming refresh token, look up
// its session (the session — not the token — is this SDK's source of
// truth, per .instructions/project-principles.md), reject if that
// session was already rotated or revoked, then issue a new access/
// refresh pair and rotate the session so the old refresh token stops
// working. That is enough security-sensitive logic that this example
// intentionally leaves it as a clearly-marked gap rather than guessing
// at it — returning a structured error here, instead of the panic this
// used to be, so a client gets a normal 501 response instead of an
// unhandled crash recovered only by the HTTP server's Recovery
// middleware.
func (s *AuthService) TokenRefresh(ctx context.Context, req model.TokenRefresh) (res model.TokenRefresh, err error) {
	return model.TokenRefresh{}, errors.NotImplemented(errors.String("token refresh is not implemented"), s.appConfig.AppName, "5010", "token refresh is not available yet")
}

func (s *AuthService) generateJwtToken(ctx context.Context, userData *pbUser.Response) (res model.Token, err error) {

	userId := userData.UserId
	sessionID := userId + ":" + userData.DevicesIdentifier + strings.ReplaceAll(uuid.New().String(), "-", "")
	expiresAccessTokenAt := time.Now().Add(time.Minute * 5)
	expiresRefreshTokenAt := time.Now().Add(time.Hour * 24)

	kid := s.appConfig.Security.Kid.Signing
	pvk, err := jwt.B64ToPrivateKey(s.appConfig.Security.PrivateKey[kid])
	const message = "error while generating jwt token"
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, "5003", message)
	}

	token, err := jwt.Issue(s.jwtConfig, pvk, userId, sessionID, time.Until(expiresAccessTokenAt), nil)
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, "5004", message)
	}

	refreshToken, err := jwt.Issue(s.jwtConfig, pvk, userId, sessionID, time.Until(expiresRefreshTokenAt), nil)
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, "5004", message)
	}

	_ = s.session.Delete(ctx, userId)

	principal := &security.Principal{
		Subject:     userId,
		Username:    userData.Subject,
		Roles:       userData.Roles,
		Permissions: userData.Permissions,
		Claims: map[string]interface{}{
			"email":             userData.Email,
			"fullName":          userData.FullName,
			"displayName":       userData.DisplayName,
			"status":            userData.Status,
			"phoneNumber":       userData.PhoneNumber,
			"avatarUrl":         userData.AvatarUrl,
			"locale":            userData.Locale,
			"timezone":          userData.Timezone,
			"devicesIdentifier": userData.DevicesIdentifier,
			"platform":          userData.Platform,
			"deviceName":        userData.DeviceName,
			"pushToken":         userData.PushToken,
		},
	}
	_, err = s.session.Create(ctx, sessionID, principal, time.Until(expiresAccessTokenAt.Add(1*time.Minute)))
	if err != nil {
		return model.Token{}, errors.InternalServerError(err, s.appConfig.AppName, "5005", "error creating session")
	}

	return model.Token{
		AccessToken:  token,
		RefreshToken: refreshToken,
	}, nil

}
