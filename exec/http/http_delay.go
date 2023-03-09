package http

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type DelayHttpActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDelayHttpActionCommandSpec() spec.ExpActionCommandSpec {
	return &DelayHttpActionCommandSpec{
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
					Desc:     "sleep time, unit is millisecond",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "target",
					Desc:     "HTTP target: Request or Response",
					Required: false,
				},
			},
			ActionExample: `
# Create a http2 10000(10s) delay experiment
blade create http2 delay --url https://www.taobao.com --time 10000

# Create a http2 10000(10s) delay request
blade create http2 delay --url https://www.taobao.com --target request --time 10000

# Create a http2 10000(10s) delay response
blade create http2 delay --url https://www.taobao.com --target response --time 10000`,
			ActionExecutor:   &HttpDelayExecutor{},
			ActionCategories: []string{category.SystemHttp},
		},
	}
}

func (*DelayHttpActionCommandSpec) Name() string {
	return "delay"
}

func (*DelayHttpActionCommandSpec) Aliases() []string {
	return []string{"d"}
}

func (*DelayHttpActionCommandSpec) ShortDesc() string {
	return "delay url"
}

func (impl *DelayHttpActionCommandSpec) LongDesc() string {
	if impl.ActionLongDesc != "" {
		return impl.ActionLongDesc
	}
	return "delay http2 by url"
}

// HttpDelayExecutor for action
type HttpDelayExecutor struct {
	channel spec.Channel
}

func (*HttpDelayExecutor) Name() string {
	return "delay"
}

func (impl *HttpDelayExecutor) SetChannel(channel spec.Channel) {
	impl.channel = channel
}

func (impl *HttpDelayExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if impl.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return impl.stop(ctx, uid)
	}
	urlStr := model.ActionFlags["url"]
	if urlStr == "" {
		log.Errorf(ctx, "url-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "url")
	}
	if !strings.Contains(urlStr, "https://") {
		log.Errorf(ctx, "url is not unsupported protocol scheme")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "url")
	}

	t := model.ActionFlags["time"]
	if t == "" {
		log.Errorf(ctx, "time-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "time")
	}
	t1, err := strconv.Atoi(t)
	if err != nil {
		log.Errorf(ctx, "time %v it must be a positive integer", t1)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "time", t1, "time must be a positive integer")
	}

	target := model.ActionFlags["target"]
	return impl.start(ctx, urlStr, t1, target)
}

func (impl *HttpDelayExecutor) start(ctx context.Context, url string, t int, target string) *spec.Response {
	switch target {
	case "request", "response":
		return impl.GetTargetDelay(ctx, url, t, target)
	default:
		time.Sleep(time.Duration(t) * time.Millisecond)
		return impl.channel.Run(ctx, "curl", url)
	}
}

func (impl *HttpDelayExecutor) GetTargetDelay(ctx context.Context, url string, t int, target string) *spec.Response {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Errorf(ctx, "get-request-url-failed", err)
		return spec.ReturnFail(spec.ActionNotSupport, fmt.Sprintf("get Request failed %s ", target))
	}
	duration := time.Duration(t) * time.Millisecond
	if target == "request" {
		time.Sleep(duration)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf(ctx, "get-client-url-failed", err)
		return spec.ReturnFail(spec.ActionNotSupport, fmt.Sprintf("get client Request failed %s ", target))
	}
	defer resp.Body.Close()
	if target == "response" {
		time.Sleep(duration)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf(ctx, "Failed to read response body", err)
		return spec.ReturnFail(spec.ActionNotSupport, fmt.Sprintf("get client response body failed %s ", string(body)))
	}

	if resp != nil {
		if resp.StatusCode == 200 {
			return spec.ReturnSuccess(resp.StatusCode)
		}
	}
	return spec.ReturnFail(spec.ParameterRequestFailed, fmt.Sprintf("get response failed %s ", resp.StatusCode))
}

func (impl *HttpDelayExecutor) stop(ctx context.Context, uid string) *spec.Response {
	return exec.Destroy(ctx, impl.channel, uid)
}
