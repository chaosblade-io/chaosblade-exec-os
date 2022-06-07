package network

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/tc"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"net"
	"strconv"
	"time"
)

var SendNetworkBin = "chaos_send"

type SendActionSpec struct {
	spec.BaseExpActionCommandSpec
}

type TCPClient struct {
	conn net.Conn
}

func (t *TCPClient) Send(ctx context.Context, str string) bool {
	n, err := t.conn.Write([]byte(str))
	if err != nil || n <= 0 {
		log.Errorf(ctx, "TCP Client Send Error: %v\n", err)
		fmt.Println(ctx, "TCP Client Send Error: %v\n", err)
		return false
	}
	return true
}

func NewTCPClient(addr string) (*TCPClient, error) {
	dialer := net.Dialer{Timeout: 20 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	return &TCPClient{conn: conn}, err
}

func NewSendActionSpec() spec.ExpActionCommandSpec {
	return &SendActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "destination-ip",
					Desc:     "Destination ip",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "remote-port",
					Desc:     "Remote port",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "quantity",
					Desc:     "Quantity",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "type",
					Desc:     "Type, Optional value: loss, duplicate, reorder, corrupt, delay",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "duration",
					Desc:     "Sending time, decide how long to send these data packets",
					Required: false,
				},
			},
			ActionExecutor: &NetworkSendExecutor{},
			ActionExample: `
# Send tcp packets to verify network tc
blade verify network send --destination-ip 127.0.0.1 --remote-port 9001 --quantity 100 --type loss
blade verify network send --destination-ip 127.0.0.1 --remote-port 9001 --quantity 100 --type delay`,
			ActionPrograms:   []string{SendNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*SendActionSpec) Name() string {
	return "send"
}

func (*SendActionSpec) Aliases() []string {
	return []string{}
}

func (*SendActionSpec) ShortDesc() string {
	return "Send tcp packets"
}

func (s *SendActionSpec) LongDesc() string {
	if s.ActionLongDesc != "" {
		return s.ActionLongDesc
	}
	return "Send tcp packets"
}

type NetworkSendExecutor struct {
	channel spec.Channel
}

func (nse *NetworkSendExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		log.Errorf(ctx, "Send only supports create or verify")
		return spec.ReturnFail(spec.CommandIllegal, "Send only supports create or verify")
	}

	var duration int
	var quantity int
	var err error
	destinationIp := model.ActionFlags["destination-ip"]
	remotePort := model.ActionFlags["remote-port"]
	quantityStr := model.ActionFlags["quantity"]
	durationStr := model.ActionFlags["duration"]
	sendType := model.ActionFlags["type"]

	if sendType != tc.Loss && sendType != tc.Duplicate && sendType != tc.Reorder && sendType != tc.Corrupt && sendType != tc.Delay {
		log.Errorf(ctx, "`%s`: type is illegal, it must be (loss | duplicate | reorder | corrupt | delay)", sendType)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "type", sendType,
			"it must be (loss | duplicate | reorder | corrupt | delay)")
	}
	if destinationIp == "" {
		log.Errorf(ctx, "destination-ip is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "destination-ip")
	}
	if remotePort == "" {
		log.Errorf(ctx, "remote-port is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "remote-port")
	}
	if quantityStr == "" {
		quantity = -1
	} else {
		quantity, err = strconv.Atoi(quantityStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: quantity is illegal, it must be a positive integer", quantityStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "quantity", quantityStr, "it must be a positive integer")
		}
	}

	if durationStr != "" {
		var err error
		duration, err = strconv.Atoi(durationStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: duration is illegal, it must be a positive integer", durationStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "duration", durationStr,
				"it must be a positive integer")
		}
		if duration <= 0 {
			log.Errorf(ctx, "`%s`: duration is illegal, it must be a positive integer and not less than 0", durationStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "duration", durationStr,
				"it must be a positive integer and not less than 0")
		}
	} else {
		duration = 3
	}
	return nse.start(ctx, destinationIp, remotePort, quantity, sendType, duration)
}

func (nse *NetworkSendExecutor) SetChannel(channel spec.Channel) {
	nse.channel = channel
}

func (*NetworkSendExecutor) Name() string {
	return "send"
}

func (nse *NetworkSendExecutor) start(ctx context.Context, ip string, port string, quantity int, sendType string, duration int) *spec.Response {
	tcpClient, err := NewTCPClient(fmt.Sprintf("%s:%s", ip, port))
	if err != nil {
		log.Errorf(ctx, "TCP client init failed", err.Error())
		return spec.ReturnFail(spec.TcpClientInitFailed, err.Error())
	}

	var intervalMs int
	var count int
	var msg string

	if quantity == -1 {
		intervalMs = 100
		quantity = duration * 1000 / intervalMs
	} else {
		intervalMs = int(float64(duration) / float64(quantity) * 1000)
	}
	if sendType != tc.Delay {
		msg = generateHandShakeMsg()
	} else {
		msg = generateTimeStampMsg()
	}

	for i := 0; i < quantity; i++ {
		if tcpClient.Send(ctx, msg) {
			count++
		}
		time.Sleep(time.Duration(intervalMs) * time.Millisecond)
	}

	return spec.ReturnSuccess(count)
}

func generateTimeStampMsg() string {
	return fmt.Sprintf("%s:%d", "chaos", time.Now().UnixNano()/1000000)
}

func generateHandShakeMsg() string {
	return fmt.Sprintf("%s:%s %s", "chaos", SHAKEHAND, time.Now().UnixNano()/1000000)
}
