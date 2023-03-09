package http

import (
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type HttpCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewHttpCommandModelSpec() spec.ExpModelCommandSpec {
	return &HttpCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpFlags: []spec.ExpFlagSpec{},
			ExpActions: []spec.ExpActionCommandSpec{
				NewDelayHttpActionCommandSpec(),
				NewRequestHttpActionCommandSpec(),
				NewTimeOutHttpActionCommandSpec(),
			},
		},
	}
}

func (*HttpCommandModelSpec) Name() string {
	return "http2"
}

func (*HttpCommandModelSpec) ShortDesc() string {
	return "Http2 experiment"
}

func (*HttpCommandModelSpec) LongDesc() string {
	return "Http2 experiment, for example, http2 request"
}
