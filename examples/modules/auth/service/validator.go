package service

import (
	"fmt"
	"strings"

	"github.com/agilsyofian/gosdk/example/modules/auth/domain"
)

// ValidateAuthCommand re-checks the fields LoginPassword actually needs,
// independently of the HTTP DTO's `binding:"required"` tags
// (handler/httphandler/http_dto.go). Two reasons this exists as its own step rather
// than trusting the transport layer:
//
//   - DeviceIdentifier never goes through DTO binding — it is populated
//     from request metadata (metadata.FromContext) after binding runs —
//     so this is the only validation it gets.
//   - A service should fail safe on its own inputs rather than assume
//     every caller is a gin handler that applied the same tags.
func ValidateAuthCommand(cmd domain.Login) error {
	if strings.TrimSpace(cmd.Username) == "" {
		return fmt.Errorf("username must not be blank")
	}
	if strings.TrimSpace(cmd.Password) == "" {
		return fmt.Errorf("password must not be blank")
	}
	if strings.TrimSpace(cmd.DeviceIdentifier) == "" {
		return fmt.Errorf("device identifier must not be blank")
	}
	return nil
}
