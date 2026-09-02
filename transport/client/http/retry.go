package http

import (
	"context"
	"net/http"

	"github.com/agilsyofian/gosdk/resilience/retry"
)

func Retry(p *retry.Policy) Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			var resp *http.Response
			var err error
			execErr := p.Execute(ctx, func(ctx context.Context) error {
				resp, err = next(ctx, req)
				return err
			})
			if execErr != nil {
				return nil, execErr
			}
			return resp, err
		}
	}
}
