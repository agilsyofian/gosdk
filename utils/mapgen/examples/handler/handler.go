// Package handler demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=model.Employee:*employeev1.Employee,model.Address:*employeev1.Address -converter-pkg=../lib/converters
package handler

import (
	_ "github.com/agilsyofian/utils/mapgen/examples/gen/employeev1"
	_ "github.com/agilsyofian/utils/mapgen/examples/model"
)
