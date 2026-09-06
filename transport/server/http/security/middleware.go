package security

import (
	"errors"

	coreErr "github.com/agilsyofian/gosdk/core/errors"

	"github.com/agilsyofian/gosdk/security"
	"github.com/gin-gonic/gin"
)

// Middleware returns the gin.HandlerFunc that performs authentication
// for a route (or route group).
//
// When cfg.Required is true, a request with no valid credential is
// rejected with 401. When false, the request proceeds without a
// SecurityContext in its context if no credential was presented;
// handlers are expected to check core.FromContext themselves.
func (m *Middleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, credential, err := extractCredential(c, m.config)

		if err != nil {
			if errors.Is(err, security.ErrNoCredential) && !m.config.Required {
				c.Next()
				return
			}
			m.abort(c, err)
			return
		}

		var securityContext *security.SecurityContext
		if sessionID != "" {
			securityContext, err = m.manager.AuthenticateSession(c.Request.Context(), sessionID)
		} else {
			securityContext, err = m.manager.Authenticate(c.Request.Context(), *credential)
		}

		if err != nil {
			if m.config.Required {
				m.abort(c, coreErr.Unauthorized(err, "security", "401", "Unauthorized"))
				return
			}
			c.Next()
			return
		}

		ctx := security.WithContext(c.Request.Context(), securityContext)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequirePermission returns a gin.HandlerFunc that enforces a
// permission check via SecurityManager.Authorize. It must run after
// Middleware() in the handler chain so a SecurityContext is already
// present.
func (m *Middleware) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := m.manager.Authorize(c.Request.Context(), permission); err != nil {
			m.abort(c, err)
			return
		}
		c.Next()
	}
}

// abort maps a security error to an HTTP status/body and aborts the
// request. See security's error semantics for the canonical mapping.
func (m *Middleware) abort(c *gin.Context, err error) {
	var appErr *coreErr.AppError
	if !errors.As(err, &appErr) {
		if errors.Is(err, security.ErrForbidden) {
			err = coreErr.Forbidden(err, "security", "403", "Forbidden")
		} else {
			err = coreErr.Unauthorized(err, "security", "401", "Unauthorized")
		}
	}
	_ = c.Error(err)
	c.Abort()
}
