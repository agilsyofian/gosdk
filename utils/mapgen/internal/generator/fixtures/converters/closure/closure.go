// Package closure registers a function literal, which mapgen rejects.
package closure

import (
	"strconv"

	"github.com/agilsyofian/utils/mapgen/runtime/mapper"
)

func init() {
	mapper.Register(func(v int) string { return strconv.Itoa(v) })
}
