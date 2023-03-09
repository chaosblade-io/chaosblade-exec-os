package http

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type RequestHttpActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewRequestHttpActionCommandSpec() spec.ExpActionCommandSpec {
	return &RequestHttpActionCommandSpec{
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
					Name:     "count",
					Desc:     "request the url to count",
					Required: true,
				},
			},

			ActionExample: `
# Create a http2 10000(10s) count request experiment
blade create http2 request --url https://www.taobao.com --count 10`,
			ActionExecutor:   &HttpRequestExecutor{},
			ActionCategories: []string{category.SystemHttp},
		},
	}
}

func (*RequestHttpActionCommandSpec) Name() string {
	return "request"
}

func (*RequestHttpActionCommandSpec) Aliases() []string {
	return []string{"r"}
}

func (*RequestHttpActionCommandSpec) ShortDesc() string {
	return "request url"
}

func (impl *RequestHttpActionCommandSpec) LongDesc() string {
	if impl.ActionLongDesc != "" {
		return impl.ActionLongDesc
	}
	return "request http2 by url"
}

// HttpRequestExecutor for action
type HttpRequestExecutor struct {
	channel spec.Channel
}

func (*HttpRequestExecutor) Name() string {
	return "request"
}

func (impl *HttpRequestExecutor) SetChannel(channel spec.Channel) {
	impl.channel = channel
}

func (impl *HttpRequestExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
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

	c := model.ActionFlags["count"]
	if c == "" {
		log.Errorf(ctx, "count-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "count")
	}
	c1, err := strconv.Atoi(c)
	if err != nil {
		log.Errorf(ctx, "count %v it must be a positive integer", c1)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", c1, "ti must be a positive integer")
	}
	return impl.start(ctx, urlStr, c1)
}

func (impl *HttpRequestExecutor) start(ctx context.Context, url string, c int) *spec.Response {
	if c == 0 {
		log.Errorf(ctx, "count-is-nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "count", c)
	}

	var response *spec.Response
	var val []interface{}
	for i := 0; i < c; i++ {
		response = impl.channel.Run(ctx, "curl", fmt.Sprintf("%s", url))
		val = append(val, response.Result)
	}
	if response == nil {
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, "response-nil")
	}
	return spec.ReturnSuccess(val)
}

func (impl *HttpRequestExecutor) stop(ctx context.Context, uid string) *spec.Response {
	return exec.Destroy(ctx, impl.channel, uid)
}
