package interceptor

import (
	"context"
	"net/http"

	"github.com/agilsyofian/gosdk/example/app/config"
	transportHttp "github.com/agilsyofian/gosdk/transport/client/http"
)

func BasicAuth(appConfig config.AppConfig) transportHttp.Middleware {
	return func(next transportHttp.RoundTripper) transportHttp.RoundTripper {
		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			req.SetBasicAuth(appConfig.ClientInterceptor.HttpBin.Username, appConfig.ClientInterceptor.HttpBin.Password)
			return next(ctx, req)
		}
	}
}
