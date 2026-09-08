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

		traceID := tracing.TraceID(c.Request.Context())
		if traceID == "" {
			if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
				traceID = span.SpanContext().TraceID().String()
			}
		}
		if traceID == "" {
			traceID = c.GetHeader("x-trace-id")
		}
		if traceID == "" {
			traceID = c.Writer.Header().Get("x-trace-id")
		}

		sessionID := tracing.SessionID(c.Request.Context())
		if sessionID == "" {
			sessionID = c.GetHeader("x-session-id")
		}
		if sessionID == "" {
			sessionID = c.Writer.Header().Get("x-session-id")
		}

		spanID := trace.SpanContextFromContext(c.Request.Context()).SpanID().String()
		if spanID == "0000000000000000" {
			spanID = c.GetHeader("x-span-id")
		}
		if spanID == "0000000000000000" {
			spanID = c.Writer.Header().Get("x-span-id")
		}

		payload := logger.LogPayload{
			Timestamp: time.Now(),
			AppName:   appName,
			Type:      "ERROR",
			Method:    c.Request.Method,
			IpAddress: c.ClientIP(),
			Metadata:  c.Request.Header,
			TraceId:   traceID,
			SpanId:    spanID,
			SessionId: sessionID,
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
			payload.Status = appErr.Status.Int()

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
			payload.Status = http.StatusBadRequest

			c.JSON(http.StatusBadRequest, resp)
			return
		}

		payload.Status = http.StatusInternalServerError
		c.JSON(http.StatusInternalServerError, resp)
	}
}
