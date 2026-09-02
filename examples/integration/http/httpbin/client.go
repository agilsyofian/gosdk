package httpbin

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/integration/interceptor"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/transport/client/http"
)

type IClientHttpBin interface {
	TestPost(ctx context.Context, req Request) (Response, error)
	SimulateError(ctx context.Context, code int) (any, error)
}

type ClientHttpBin struct {
	cfg    http.Config
	client *http.Client
}

func NewClientHttpBin(cfg http.Config, l logger.Logger, appConfig config.AppConfig) IClientHttpBin {

	client := http.New(cfg.BaseURL, cfg.Timeout, l, cfg)
	client.Use(interceptor.BasicAuth(appConfig))

	return &ClientHttpBin{
		cfg:    cfg,
		client: client,
	}

}

func (t ClientHttpBin) TestPost(ctx context.Context, req Request) (res Response, err error) {
	err = t.client.Execute(ctx, "POST", "/post", req, &res)
	return res, err
}

// SimulateError calls https://httpbin.org/status/{code}, which always
// responds with that HTTP status and an empty body. It exists to
// exercise this SDK's resilience layer (retry/circuit-breaker —
// http_integrations config) against real error responses: try it with
// 408 or 429 to see a retry kick in, or several 500s in a row to trip
// the circuit breaker.
func (t ClientHttpBin) SimulateError(ctx context.Context, code int) (any, error) {
	var resp any
	err := t.client.Execute(ctx, "GET", fmt.Sprintf("/status/%d", code), nil, &resp)
	return resp, err
}
