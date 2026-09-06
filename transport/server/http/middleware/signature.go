package middleware

import (
	"errors"
	"fmt"
	"net/http"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/security/signature"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

// SignatureConfig configures the Signature verification and response signing middleware.
type SignatureConfig struct {
	Engine  *signature.Engine
	AppName string
}

// SignatureOption configures the Signature middleware.
type SignatureOption func(*signature.Config)

// Signature returns a Gin middleware that verifies incoming request signatures and optionally signs outgoing responses.
func Signature(provider signature.SecretProvider, opts ...signature.Option) gin.HandlerFunc {
	engine := signature.NewEngine(provider, opts...)
	appName := "server"

	return func(c *gin.Context) {
		secret, ok := verifyIncomingRequest(c, engine, appName)
		if !ok {
			return
		}

		if !engine.Config().SignResponse {
			c.Next()
			return
		}

		recorder := captureResponseBody(c)
		c.Next()

		signOutgoingResponse(c, engine, secret, recorder)
	}
}

func verifyIncomingRequest(c *gin.Context, engine *signature.Engine, appName string) ([]byte, bool) {
	body := captureRequestBody(c)
	keyID, secret, err := engine.VerifyRequest(c.Request.Context(), c.Request, body)
	if err != nil {
		handleSignatureError(c, appName, err)
		return nil, false
	}

	if keyID != "" {
		c.Set("client_id", keyID)
		c.Set("signature_key_id", keyID)
	}

	return secret, true
}

func signOutgoingResponse(c *gin.Context, engine *signature.Engine, secret []byte, recorder *bodyLogWriter) {
	if len(secret) == 0 {
		return
	}

	status := c.Writer.Status()
	sig, ts, err := engine.SignResponse(status, recorder.body.Bytes(), secret)
	if err == nil {
		c.Header(engine.Config().ResponseSignatureHeader, sig)
		c.Header(engine.Config().ResponseTimestampHeader, ts)
	}
}

func handleSignatureError(c *gin.Context, appName string, err error) {
	statusCode := http.StatusUnauthorized
	if errors.Is(err, signature.ErrMissingSignature) ||
		errors.Is(err, signature.ErrMissingTimestamp) ||
		errors.Is(err, signature.ErrInvalidTimestamp) {
		statusCode = http.StatusBadRequest
	}

	appErr := appError.New(
		err,
		mapHTTPStatus(statusCode),
		appName,
		fmt.Sprintf("%d", statusCode),
		err.Error(),
		nil,
	)
	_ = c.Error(appErr)
	c.AbortWithStatusJSON(statusCode, dto.New(c.Request.Context(), nil).
		WithSource(appName).
		WithCode(fmt.Sprintf("%d", statusCode)).
		WithMessage(err.Error()))
}
