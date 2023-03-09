package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type TimeOutHttpActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewTimeOutHttpActionCommandSpec() spec.ExpActionCommandSpec {
	return &TimeOutHttpActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "url",
					Desc:                  "The Url of the target http2",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "time",
					Desc:     "The Time to Disconnect HTTP connection",
					Required: true,
				},
			},

			ActionExample: `
# Create a http2 1000(1s) timeout experiment
blade create http2 timeout --url https://www.taobao.com --time 1000`,
			ActionExecutor:   &HttpTimeoutExecutor{},
			ActionCategories: []string{category.SystemHttp},
		},
	}
}

func (*TimeOutHttpActionCommandSpec) Name() string {
	return "timeout"
}

func (*TimeOutHttpActionCommandSpec) Aliases() []string {
	return []string{"t"}
}

func (*TimeOutHttpActionCommandSpec) ShortDesc() string {
	return "timeout url"
}

func (impl *TimeOutHttpActionCommandSpec) LongDesc() string {
	if impl.ActionLongDesc != "" {
		return impl.ActionLongDesc
	}
	return "timeout http2 by url"
}

// HttpTimeoutExecutor for action
type HttpTimeoutExecutor struct {
	channel spec.Channel
}

func (*HttpTimeoutExecutor) Name() string {
	return "timeout"
}

func (impl *HttpTimeoutExecutor) SetChannel(channel spec.Channel) {
	impl.channel = channel
}

func (impl *HttpTimeoutExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if impl.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return impl.stop(ctx, uid)
	}

	urlStr := model.ActionFlags["url"]
	if urlStr == "" {
		log.Errorf(ctx, "url-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "url")
	}

	if !strings.Contains(urlStr, "https://") {
		log.Errorf(ctx, "url is not unsupported protocol scheme")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "url")
	}

	timeout := model.ActionFlags["time"]
	if timeout == "" {
		log.Errorf(ctx, "timeout-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "timeout")
	}
	t1, err := strconv.Atoi(timeout)
	if err != nil {
		log.Errorf(ctx, "timeout %v it must be a positive integer", t1)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "timeout", t1, "timeout must be a positive integer")
	}
	return impl.start(ctx, urlStr, t1)
}

func (impl *HttpTimeoutExecutor) start(ctx context.Context, url string, t int) *spec.Response {
	if t == 0 {
		log.Errorf(ctx, "timeout-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "timeout", t)
	}

	ctxNew, cancel := context.WithTimeout(context.Background(), time.Duration(t)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxNew, http.MethodGet, url, nil)
	if err != nil {
		log.Errorf(ctxNew, "http NewRequestWithContext creation failed:", err.Error())
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, "NewRequestWithContext timeout", t)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf(ctxNew, "Request failed:", err.Error())
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, "the interrupt HTTP connection timeout", t)
	}
	defer resp.Body.Close()

	if resp != nil {
		if resp.StatusCode == 200 {
			return spec.ReturnSuccess(resp.StatusCode)
		}
	}
	return spec.ReturnFail(spec.ParameterRequestFailed, fmt.Sprintf("get response failed %s ", resp.StatusCode))
}

func (impl *HttpTimeoutExecutor) stop(ctx context.Context, uid string) *spec.Response {
	return exec.Destroy(ctx, impl.channel, uid)
}
