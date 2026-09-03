// Package method registers a method expression, which mapgen rejects.
package method

import (
	"time"

	"github.com/agilsyofian/gosdk/utils/mapgen/runtime/mapper"
)

func init() {
	mapper.Register(time.Time.String)
}
