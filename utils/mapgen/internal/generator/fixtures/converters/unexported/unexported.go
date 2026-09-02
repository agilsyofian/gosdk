// Package unexported registers an unexported converter, which mapgen
// rejects unless the output package is the same.
package unexported

import (
	"strconv"

	"github.com/agilsyofian/utils/mapgen/runtime/mapper"
)

func init() {
	mapper.Register(format)
}

func format(v int) string {
	return strconv.Itoa(v)
}
