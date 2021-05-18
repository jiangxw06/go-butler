package http

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/prometheus"
	"time"
)

var (
	httpClient *resty.Client
)

func initClient() {
	httpClient = resty.New()
	if !env.IsProd() {
		httpClient.EnableTrace()
	}
	httpClient.SetTimeout(time.Second * 15) //默认超时时间
	//httpClient.SetDebug(true)
	httpClient.OnBeforeRequest(httpRequestMiddleware)
	httpClient.OnAfterResponse(httpResponseMiddleware)
}

func httpRequestMiddleware(client *resty.Client, req *resty.Request) error {
	//todo 根据request设置是否enableTrace
	//req.EnableTrace()
	return nil
}

func httpResponseMiddleware(client *resty.Client, resp *resty.Response) error {
	url := resp.RawResponse.Request.URL
	prometheus.HttpReqDuration(url.Host, url.Path, resp.StatusCode()).Observe(resp.Time().Seconds())
	ctx := resp.Request.Context()
	duration := resp.Time().Milliseconds()
	contexts.Logger(ctx).Debugw("http outgoing", "duration", duration, "url", url)

	return nil
}

func Request(ctx context.Context) *resty.Request {
	httpOnce.Do(initHttpConfig)
	return httpClient.R().SetContext(ctx)
}

func RequestWithTimeout(ctx context.Context, timeout time.Duration) (*resty.Request, context.CancelFunc) {
	httpOnce.Do(initHttpConfig)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return Request(ctx), cancel
}
