// Package grpc Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=model.User:*user.Response -converter-pkg=../../../../app/lib/converters
package grpc

import (
	_ "github.com/agilsyofian/gosdk/example/modules/user/model"
	_ "github.com/agilsyofian/gosdk/example/proto/user"
)
