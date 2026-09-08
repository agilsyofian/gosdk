// Package httphandler Package handler Package http Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=model.User:UserDetail -converter-pkg=../../../../app/shared/converters -ignore model.User.PasswordHash,model.User.BiometricHash
package httphandler

import (
	_ "user-service/modules/user/domain"
)
