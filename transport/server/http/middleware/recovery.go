package middleware

import (
	goError "errors"

	"github.com/agilsyofian/gosdk/errors"
	"github.com/gin-gonic/gin"
)

func Recovery(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {

		defer func() {
			if r := recover(); r != nil {
				var err error
				switch x := r.(type) {
				case error:
					err = errors.InternalServerError(x, appName, errors.StatusInternalServerError.String(), "An error occurred")
				default:
					err = errors.Unknown(goError.New("unknown error"))
				}

				_ = c.Error(err)
			}
		}()

		c.Next()

	}
}
