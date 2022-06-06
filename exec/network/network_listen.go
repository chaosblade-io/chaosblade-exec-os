package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/tc"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const ListenNetworkBin = "chaos_listennetwork"
const (
	Remote = "remote"
	Local  = "local"
)
const (
	SHAKEHAND = "SHAKEHAND"
)

// such as lost pkg number for loss, duplicate pkg number for duplicate
var listenValueInt int

// ListenValue listen result, x represents molecular, y represents denominator
type ListenValue struct {
	x int
	y int
}

var listenValue ListenValue

type ListenActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewListenActionSpec() spec.ExpActionCommandSpec {
	return &ListenActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "interface",
					Desc:     "Network interface, for example, eth0",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "type",
					Desc:     "Type, Optional value: loss, duplicate, reorder, corrupt, delay",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "direction",
					Desc:     "Listen direction, Optional value: remote | local",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "local-port",
					Desc:     "Local port, Required when direction is local",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "destination-ip",
					Desc:     "Destination ip, optional when direction is out",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "remote-port",
					Desc:     "Remote port, required when direction is remote",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "http-port",
					Desc:     "Http port, used for communication packet capture results, No need to fill in manually",
					Required: false,
				},
			},
			ActionExecutor: &NetworkListenExecutor{},
			ActionExample: `
# Listen packet loss on port 9001
blade verify network listen --interface eth0 --direction local --type loss --local-port 9001`,
			ActionPrograms:   []string{ListenNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*ListenActionSpec) Name() string {
	return "listen"
}

func (*ListenActionSpec) Aliases() []string {
	return []string{}
}

func (*ListenActionSpec) ShortDesc() string {
	return "listen tcp port"
}

func (l *ListenActionSpec) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "Listen tcp port"
}

type NetworkListenExecutor struct {
	channel spec.Channel
}

func (nle *NetworkListenExecutor) SetChannel(channel spec.Channel) {
	nle.channel = channel
}

func (*NetworkListenExecutor) Name() string {
	return "listen"
}

func (nle *NetworkListenExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		log.Errorf(ctx, "Listen only supports create or verify")
		return spec.ReturnFail(spec.CommandIllegal, "Listen only supports create or verify")
	}

	var dev string
	var localPort int
	var destinationIp string
	var remotePort int
	var listenType string
	var direction string
	var httpPort int
	var err error

	if netInterface, ok := model.ActionFlags["interface"]; ok {
		if netInterface == "" {
			log.Errorf(ctx, "interface is nil")
			return spec.ResponseFailWithFlags(spec.ParameterLess, "interface")
		}
		dev = netInterface
	}

	listenType = model.ActionFlags["type"]
	if listenType != tc.Loss && listenType != tc.Duplicate && listenType != tc.Reorder && listenType != tc.Corrupt && listenType != tc.Delay {
		log.Errorf(ctx, "`%s`: type is illegal, it must be (loss | duplicate | reorder | corrupt | delay)", listenType)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "type", listenType,
			"it must be (loss | duplicate | reorder | corrupt | delay)")
	}

	direction = model.ActionFlags["direction"]
	if direction != Remote && direction != Local {
		log.Errorf(ctx, "`%s`: direction is illegal, it must be (out | in)", direction)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "direction", direction,
			"it must be (out | in)")
	}
	if direction == Remote {
		destinationIp = model.ActionFlags["destination-ip"]
		if remotePort, err = strconv.Atoi(model.ActionFlags["remote-port"]); err != nil {
			log.Errorf(ctx, "remote-port is nil")
			return spec.ResponseFailWithFlags(spec.ParameterLess, "remote-port")
		}
	} else if direction == Local {
		if localPort, err = strconv.Atoi(model.ActionFlags["local-port"]); err != nil {
			log.Errorf(ctx, "local-port is nil")
			return spec.ResponseFailWithFlags(spec.ParameterLess, "local-port")
		}
	}

	httpPortStr := model.ActionFlags["http-port"]
	if httpPortStr == "" {
		if port, err := util.GetUnusedPort(); err != nil {
			log.Errorf(ctx, "get unused port error: %v", err.Error())
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("get unused port error: %v", err.Error()))
		} else {
			flags := fillFlags(dev, localPort, destinationIp, remotePort, listenType, direction, port)
			response := channel.NewLocalChannel().
				Run(context.Background(), "nohup", fmt.Sprintf("%s %s", path.Join(nle.channel.GetScriptPath(),
					"chaos_os"), flags))
			if !response.Success {
				log.Errorf(ctx, fmt.Sprintf("%s %s", path.Join(nle.channel.GetScriptPath(), "chaos_os"), flags))
				return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s %s", path.Join(nle.channel.GetScriptPath(), "chaos_os"), flags))
			}
			return spec.ReturnSuccess(port)
		}
	} else {
		if httpPort, err = strconv.Atoi(httpPortStr); err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "http-port", httpPortStr, "cpu-percent is illegal, it must be a positive integer")
		}
	}

	return nle.start(ctx, dev, listenType, direction, localPort, destinationIp, remotePort, httpPort)
}

func fillFlags(dev string, localPort int, destinationIp string, remotePort int, listenType string, direction string, httpPort int) string {
	flags := fmt.Sprintf(" verify network listen --interface %s --direction %s --http-port %v --type %s ", dev, direction, httpPort, listenType)
	if direction == Local {
		flags = flags + fmt.Sprintf(" --local-port %v >/dev/null 2>&1 &", localPort)
	} else {
		flags = flags + fmt.Sprintf(" --remotePort %v --destinationIp %s >/dev/null 2>&1 &", remotePort, destinationIp)
	}
	return flags
}

func (nle *NetworkListenExecutor) start(ctx context.Context, dev string, listenType string, direction string, localPort int,
	destinationIp string, remotePort int, httpPort int) *spec.Response {
	// start capture tcp packages
	response := startCapture(ctx, dev, listenType, direction, localPort, destinationIp, remotePort)
	if !response.Success {
		log.Errorf(ctx, "start capture tcp packages failed, %s", response.Err)
		return response
	}

	// start http server
	return startHttpServer(ctx, httpPort)
}

func startHttpServer(ctx context.Context, httpPort int) *spec.Response {
	http.HandleFunc("/result", func(writer http.ResponseWriter, request *http.Request) {
		marshal, _ := json.Marshal(map[string]int{"x": listenValue.x, "y": listenValue.y})
		_, err := fmt.Fprintf(writer, fmt.Sprintf("%s", string(marshal)))
		if err != nil {
			log.Errorf(ctx, err.Error())
			return
		}
	})
	http.HandleFunc("/stop", func(writer http.ResponseWriter, request *http.Request) {
		os.Exit(0)
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
	if err != nil {
		log.Errorf(ctx, "start http sever failed, %s", err.Error())
		return spec.ReturnFail(spec.OsCmdExecFailed, "start http sever failed")
	}
	return spec.ReturnSuccess(httpPort)
}

func startCapture(ctx context.Context, dev string, listenType string, direction string, localPort int, destinationIp string, remotePort int) *spec.Response {
	snapLen := int32(65535)

	ipv4Addr, err := GetInterfaceIpv4Addr(dev)
	if err != nil {
		log.Errorf(ctx, "Failed to get ipv4 address of network interface")
		return spec.ReturnFail(spec.OsCmdExecFailed, "Get ipv4 addr failed")
	}

	var filter string
	if direction == Local {
		filter = getLocalFilter(localPort, ipv4Addr)
	} else if direction == Remote {
		filter = getRemoteFilter(remotePort, destinationIp)
	}

	// start network listen handle
	handle, err := pcap.OpenLive(dev, snapLen, true, pcap.BlockForever)
	if err != nil {
		log.Errorf(ctx, "pcap open live failed: %v", err)
		return spec.ReturnFail(spec.OsCmdExecFailed, "Pcap open live failed")
	}

	// set handle filter
	if err = handle.SetBPFFilter(filter); err != nil {
		log.Errorf(ctx, "set bpf filter failed: %v", err)
		return spec.ReturnFail(spec.OsCmdExecFailed, "Set BPF filter failed")
	}

	go doCapture(ctx, handle, listenType, ipv4Addr, direction)
	return spec.Success()
}

func doCapture(ctx context.Context, handle *pcap.Handle, listenType string, ip string, direction string) {
	// 抓包
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true
	switch listenType {
	case tc.Loss:
		onLoss(ctx, packetSource, ip)
	case tc.Reorder:
		onReorder(ctx, packetSource)
	case tc.Duplicate:
		onDuplicate(ctx, packetSource)
	case tc.Corrupt:
		onCorrupt(ctx, packetSource, ip)
	case tc.Delay:
		onDelay(ctx, packetSource, ip, direction)
	}
}

func onDelay(ctx context.Context, packetSource *gopacket.PacketSource, ipAddr string, direction string) {
	// Ack number 2 timestamp
	expectedAck2Time := make(map[string]int64)

	for packet := range packetSource.Packets() {
		if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			log.Warnf(ctx, "unexpected packet")
			continue
		}

		ip := packet.NetworkLayer().(*layers.IPv4)
		tcp := packet.TransportLayer().(*layers.TCP)

		if direction == Local {
			if ip.SrcIP.String() != ipAddr {
				expectedAck := tcp.Seq + uint32(len(tcp.Payload))
				if tcp.SYN || tcp.FIN {
					expectedAck += 1
				}
				expectedAck2Time[ip.SrcIP.String()+strconv.FormatUint(uint64(expectedAck), 10)] = time.Now().UnixNano() / 1000000
			} else {
				if tcp.ACK {
					receiveTime := expectedAck2Time[ip.DstIP.String()+strconv.FormatUint(uint64(tcp.Ack), 10)]
					if receiveTime > 0 {
						delay := time.Now().UnixNano()/1000000 - receiveTime
						listenValue.x += int(delay)
						listenValue.y++
					}
				}
			}
		} else {
			if ip.SrcIP.String() != ipAddr {
				continue
			} else {
				payload := strings.TrimSpace(fmt.Sprintf("%s", tcp.Payload))
				if strings.HasPrefix(payload, "chaos") {
					sendTime, err := strconv.ParseInt(strings.Split(payload, ":")[1], 10, 64)
					if err != nil {
						continue
					}
					realSendTime := time.Now().UnixNano() / 1000000
					delayTime := int(realSendTime - sendTime)
					listenValue.x += delayTime
					listenValue.y++
				}
			}
		}
	}
}

func onDuplicate(ctx context.Context, packetSource *gopacket.PacketSource) {
	for packet := range packetSource.Packets() {
		if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			log.Warnf(ctx, "unexpected packet")
			continue
		}

		// Custom TCP protocol, for example: "chaos:SHAKEHAND"
		tcp := packet.TransportLayer().(*layers.TCP)
		payload := fmt.Sprintf("%s", tcp.Payload)
		keyValuePair := strings.Split(payload, ":")
		if !strings.HasPrefix(payload, "chaos") || len(keyValuePair) != 2 {
			log.Warnf(ctx, "unexpected packet")
			continue
		}

		seqMap := make(map[uint32]bool)
		if strings.HasPrefix(keyValuePair[1], SHAKEHAND) {
			if seqMap[tcp.Seq] == true {
				listenValueInt++
			} else {
				seqMap[tcp.Seq] = true
			}
		}
	}
}

func onReorder(ctx context.Context, packetSource *gopacket.PacketSource) {
	for packet := range packetSource.Packets() {
		if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			log.Warnf(ctx, "unexpected packet")
			continue
		}

		// Custom TCP protocol, for example: "chaos:SHAKEHAND"
		tcp := packet.TransportLayer().(*layers.TCP)
		payload := fmt.Sprintf("%s", tcp.Payload)
		keyValuePair := strings.Split(payload, ":")
		if !strings.HasPrefix(payload, "chaos") || len(keyValuePair) != 2 {
			log.Warnf(ctx, "unexpected packet")
			continue
		}

		var maxSeq uint32
		if strings.HasPrefix(keyValuePair[1], SHAKEHAND) {
			if tcp.Seq > maxSeq {
				maxSeq = tcp.Seq
			} else {
				listenValueInt++
			}
		}
	}
}

/**
For loss listen verify, we just need to count received pkg number, then box calculate with send pkg number
*/
func onLoss(ctx context.Context, packetSource *gopacket.PacketSource, ipAddr string) {
	for packet := range packetSource.Packets() {
		if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			log.Warnf(ctx, "unexpected packet")
			continue
		}
		ip := packet.NetworkLayer().(*layers.IPv4)

		if ipAddr == ip.SrcIP.String() {
			listenValue.x++
		} else {
			listenValue.y++
		}
	}
}

func onCorrupt(ctx context.Context, packetSource *gopacket.PacketSource, dev string) {
	onLoss(ctx, packetSource, dev)
}

func GetInterfaceIpv4Addr(interfaceName string) (addr string, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	if ief, err = net.InterfaceByName(interfaceName); err != nil {
		return "", err
	}
	if addrs, err = ief.Addrs(); err != nil {
		return "", err
	}
	for _, addr := range addrs { // get ipv4 address
		if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
			break
		}
	}
	if ipv4Addr == nil {
		return "", errors.New(fmt.Sprintf("interface %s don't have an ipv4 address", interfaceName))
	}
	return ipv4Addr.String(), nil
}

func getLocalFilter(port int, ip string) string {
	return fmt.Sprintf("tcp and (src host %v and src port %v) or (dst host %v and dst port %v)", ip, port, ip, port)
}

func getRemoteFilter(port int, ip string) string {
	if ip != "" {
		return fmt.Sprintf("(src host %v and src port %v) or (dst host %v and dst port %v)", ip, port, ip, port)
	} else {
		return fmt.Sprintf("tcp and port %v", port)
	}
}
