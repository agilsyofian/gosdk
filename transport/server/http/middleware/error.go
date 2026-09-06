package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel/trace"
)

func Error(l logger.Logger, appName string, showErrorDetail bool) gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		payload := logger.LogPayload{
			Timestamp: time.Now(),
			AppName:   appName,
			Type:      "ERROR",
			Method:    c.Request.Method,
			IpAddress: c.ClientIP(),
			Metadata:  c.Request.Header,
			TraceId:   tracing.TraceID(c.Request.Context()),
			SpanId:    trace.SpanContextFromContext(c.Request.Context()).SpanID().String(),
			SessionId: tracing.SessionID(c.Request.Context()),
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			Request:   "-",
			Response:  "-",
			Duration:  0,
			Error:     c.Errors.Last().Err,
		}

		if details, ok := c.Get(PanicDetailsKey); ok {
			payload.Error = details
		}

		if logged, _ := c.Get(PanicLoggedKey); logged != true {
			defer func() {
				l.Error(
					c.Request.Context(),
					fmt.Sprintf("%s %s", "ERROR", appName),
					payload,
				)
			}()
		}

		err := c.Errors.Last().Err

		resp := dto.New(c.Request.Context(), nil).
			WithSource(appError.GeneralSource).
			WithCode(appError.GeneralCode)

		if showErrorDetail {
			resp.Error = err.Error()
		}

		var appErr *appError.AppError
		if errors.As(err, &appErr) {

			resp.Source = appErr.Source
			resp.Code = appErr.Code
			resp.Message = appErr.Message
			resp.Result = appErr.Result

			payload.Error = appErr.Error()

			c.JSON(appErr.Status.Int(), resp)
			return
		}

		var validationErr validator.ValidationErrors
		if errors.As(err, &validationErr) {

			fields := make(map[string]string)

			for _, fe := range validationErr {
				fields[fe.Field()] = fe.Tag()
			}

			resp.Code = "0001"
			resp.Message = "Validation failed"
			resp.Result = fields

			payload.Error = fields

			c.JSON(http.StatusBadRequest, resp)
			return
		}

		c.JSON(http.StatusInternalServerError, resp)
	}
}
