package security

import (
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/authorization"
)

func Manager(
	jwtAuth security.Authenticator,
	session security.SessionManager,
) security.SecurityManager {

	registry := security.NewRegistry()
	registry.Register(jwtAuth)

	securityManager, err := security.NewManager(security.ManagerConfig{
		Authenticator: registry,
		Sessions:      session,
		Authorizer:    authorization.NewPermissionAuthorizer(nil),
	})
	if err != nil {
		panic(err)
	}

	return securityManager
}
