package middleware

import (
	"fmt"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/gin-gonic/gin"
)

func Recovery(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {

		defer func() {
			if r := recover(); r != nil {
				var err error
				switch x := r.(type) {
				case error:
					err = x
				default:
					err = fmt.Errorf("%v", x)
				}

				appErr := errors.InternalServerError(
					err,
					appName,
					errors.StatusInternalServerError.String(),
					"An error occurred",
				)

				_ = c.Error(appErr)
				c.Abort()
			}
		}()

		c.Next()

	}
}
