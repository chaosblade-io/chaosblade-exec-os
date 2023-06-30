package windows

import "github.com/chaosblade-io/chaosblade-spec-go/spec"

var commFlags = []spec.ExpFlagSpec{
	&spec.ExpFlag{
		Name:     "direction",
		Desc:     "direction, the value is inbound or outbound",
		Required: true,
	},
	&spec.ExpFlag{
		Name: "dst-port",
		Desc: "dst-port",
	},
	&spec.ExpFlag{
		Name: "src-port",
		Desc: "src-port",
	},
	&spec.ExpFlag{
		Name: "src-ip",
		Desc: "src-ip",
	},
	&spec.ExpFlag{
		Name: "dst-ip",
		Desc: "dst-ip",
	},
	&spec.ExpFlag{
		Name: "exclude-dst-port",
		Desc: "",
	},
	&spec.ExpFlag{
		Name: "exclude-src-port",
		Desc: "",
	},
	&spec.ExpFlag{
		Name: "exclude-dst-ip",
		Desc: "",
	},
}
