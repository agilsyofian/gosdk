// Package http Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=model.User:UserDetail -converter-pkg=../../../../app/lib/converters -ignore model.User.PasswordHash,model.User.BiometricHash
package http

import (
	_ "github.com/agilsyofian/gosdk/example/modules/user/model"
)
