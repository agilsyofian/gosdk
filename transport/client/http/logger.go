package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel/trace"
)

type CombinedRequest struct {
	Body  string            `json:"body"`
	Query map[string]string `json:"query"`
}

type CombinedResponse struct {
	Body string `json:"body"`
}

func Logger(l logger.Logger, appName string) Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			start := time.Now()

			reqBody := captureRequestBody(req)
			reqQuery := captureQueryParams(req)

			fullRequest, _ := json.Marshal(CombinedRequest{
				Body:  string(reqBody),
				Query: reqQuery,
			})

			resp, err := next(ctx, req)

			payload := logger.LogPayload{
				Timestamp: start,
				AppName:   appName,
				Type:      "INTEGRATION",
				Method:    req.Method,
				Metadata:  req.Header,
				TraceId:   coreContext.TraceID(ctx),
				SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
				SessionId: coreContext.SessionID(ctx),
				Path:      req.URL.String(),
				Request:   string(fullRequest),
				Duration:  time.Since(start).Milliseconds(),
			}

			if err != nil {
				payload.Error = err.Error()
				errResp := map[string]any{
					"code":    "CLIENT_ERROR",
					"message": err.Error(),
				}
				b, _ := json.Marshal(errResp)
				fullResponse, _ := json.Marshal(CombinedResponse{
					Body: string(b),
				})
				payload.Response = string(fullResponse)
				l.Error(ctx, fmt.Sprintf("CLIENT %s %s", req.Method, req.URL.Path), payload)
				return resp, err
			}

			resBody := captureResponseBody(resp)
			payload.Status = resp.StatusCode
			fullResponse, _ := json.Marshal(CombinedResponse{
				Body: string(resBody),
			})
			payload.Response = string(fullResponse)

			logFunc := l.Info
			if payload.Status >= 500 {
				logFunc = l.Error
			} else if payload.Status >= 400 {
				logFunc = l.Warn
			}

			logFunc(
				ctx,
				fmt.Sprintf("CLIENT %s %s", req.Method, req.URL.Path),
				payload,
			)

			return resp, nil
		}
	}
}

// captureRequestBody reads and resets the request body for further use.
func captureRequestBody(req *http.Request) []byte {
	if req.Body == nil {
		return nil
	}

	bodyBytes, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes
}

// captureResponseBody reads and resets the response body for further use.
func captureResponseBody(resp *http.Response) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes
}

// Helper function to extract query parameters from the request
func captureQueryParams(req *http.Request) map[string]string {
	query := req.URL.Query()
	if len(query) == 0 {
		return nil
	}

	queryParams := make(map[string]string)
	for key, values := range query {
		queryParams[key] = strings.Join(values, ",")
	}

	return queryParams
}
