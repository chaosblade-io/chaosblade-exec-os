package network

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/tc"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"regexp"
	"strconv"
	"strings"
)

var PingNetworkBin = "chaos_pingnetwork"

type PingParams struct {
	experimentType  string
	destinationIp   string
	quantity        int
	duration        int
	expectedPercent int
	expectedDelay   int
	tolerance       int
}

type PingActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewPingActionSpec() spec.ExpActionCommandSpec {
	return &PingActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "destination-ip",
					Desc:     "Destination ip",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "quantity",
					Desc:     "Quantity of ping packets",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "duration",
					Desc:     "Duration",
					Required: false,
					Default:  "3",
				},
				&spec.ExpFlag{
					Name:     "type",
					Desc:     "Type",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "expected-percent",
					Desc:     "Expected Percent, only valid when type is loss or duplicate or reorder or corrupt",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "expected-delay",
					Desc:     "Expected Delay, only valid when type is delay",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "tolerance",
					Desc:     "Tolerance, expected-percent 50 tolerance 10 represents 40% - 60%",
					Required: false,
				},
			},
			ActionExecutor: &NetworkPingExecutor{},
			ActionExample: `
# Verify that the network loss experiment is valid
blade verify network ping --type loss --destination-ip 11.11.11.11 --expected-percent 80 --tolerance 10

# Verify that the network duplicate experiment is valid
blade verify network ping --type duplicate --destination-ip 11.11.11.11 --expected-percent 80 --tolerance 10

# Verify that the network reorder experiment is valid
blade verify network ping --type reorder --destination-ip 11.11.11.11 --expected-percent 80 --tolerance 10

# Verify that the network corrupt experiment is valid
blade verify network ping --type corrupt --destination-ip 11.11.11.11 --expected-percent 80 --tolerance 10

# Verify that the network delay experiment is valid
blade verify network ping --type delay --destination-ip 11.11.11.11 --expected-delay 3000 --tolerance 1000`,
			ActionPrograms:   []string{PingNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*PingActionSpec) Name() string {
	return "ping"
}

func (*PingActionSpec) Aliases() []string {
	return []string{}
}

func (*PingActionSpec) ShortDesc() string {
	return "ping experiment"
}

func (p *PingActionSpec) LongDesc() string {
	if p.ActionLongDesc != "" {
		return p.ActionLongDesc
	}
	return "Ping experiment"
}

type NetworkPingExecutor struct {
	channel spec.Channel
}

func (*NetworkPingExecutor) Name() string {
	return "ping"
}

func (npe *NetworkPingExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"ping"}
	if response, ok := npe.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		log.Errorf(ctx, "Ping only supports create or verify")
		return spec.ReturnFail(spec.CommandIllegal, "Ping only supports create or verify")
	}

	if pingParams, response := checkPingParams(ctx, model.ActionFlags); !response.Success {
		return response
	} else {
		return npe.start(ctx, pingParams)
	}
}

func checkPercent(percent float64, expectedPercent int, tolerance int) bool {
	return percent >= float64(expectedPercent-tolerance) && percent <= float64(expectedPercent+tolerance)
}

func checkPingParams(ctx context.Context, actionFlags map[string]string) (*PingParams, *spec.Response) {
	pingParams := &PingParams{}
	var quantity int
	var duration int
	var expectedPercent int
	var expectedDelay int
	var tolerance int

	experimentType := actionFlags["type"]
	destinationIp := actionFlags["destination-ip"]
	quantityStr := actionFlags["quantity"]
	durationStr := actionFlags["duration"]
	expectedPercentStr := actionFlags["expected-percent"]
	expectedDelayStr := actionFlags["expected-delay"]
	toleranceStr := actionFlags["tolerance"]

	if experimentType == "" || destinationIp == "" {
		log.Errorf(ctx, "type|destinationIp is nil")
		return pingParams, spec.ResponseFailWithFlags(spec.ParameterLess, "type|destination-ip")
	}

	if experimentType != tc.Loss && experimentType != tc.Duplicate && experimentType != tc.Reorder && experimentType != tc.Corrupt && experimentType != tc.Delay {
		log.Errorf(ctx, "`%s`: type is illegal, it must be (loss | duplicate | reorder | corrupt | delay)", experimentType)
		return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "type", experimentType,
			"it must be (loss | duplicate | reorder | corrupt | delay)")
	}

	if experimentType == tc.Delay {
		var err error
		expectedDelay, err = strconv.Atoi(expectedDelayStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: expectedDelay is illegal, it must be a positive integer", expectedDelay)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "expected-delay", expectedDelayStr, "it must be a positive integer")
		}
		if expectedDelay <= 0 {
			log.Errorf(ctx, "`%s`: expectedDelay is illegal, it must be a positive integer and not less than 0", expectedDelay)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "expected-delay", expectedDelayStr,
				"it must be a positive integer and not less than 0")
		}
	} else {
		var err error
		expectedPercent, err = strconv.Atoi(expectedPercentStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: expectedPercent is illegal, it must be a positive integer", expectedPercent)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "expected-percent", expectedPercentStr, "it must be a positive integer")
		}
		if expectedPercent < 0 || expectedPercent > 100 {
			log.Errorf(ctx, "`%s`: expectedPercent is illegal, it must be a positive integer and between 0 and 100", expectedDelay)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "expected-percent", expectedPercentStr,
				"it must be a positive integer and between 0 and 100")
		}
	}

	if quantityStr != "" {
		var err error
		quantity, err = strconv.Atoi(quantityStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: quantity is illegal, it must be a positive integer", quantityStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "quantity", quantityStr, "it must be a positive integer")
		}
		if quantity <= 0 {
			log.Errorf(ctx, "`%s`: quantity is illegal, it must be a positive integer and not less than 0", quantityStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "quantity", quantityStr,
				"it must be a positive integer and not less than 100")
		}
	} else {
		quantity = 100
	}

	if durationStr != "" {
		var err error
		duration, err = strconv.Atoi(durationStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: duration is illegal, it must be a positive integer", durationStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "duration", durationStr,
				"it must be a positive integer")
		}
		if duration <= 0 {
			log.Errorf(ctx, "`%s`: duration is illegal, it must be a positive integer and not less than 0", durationStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "duration", durationStr,
				"it must be a positive integer and not less than 0")
		}
	} else {
		duration = 3
	}

	if toleranceStr != "" {
		var err error
		tolerance, err = strconv.Atoi(toleranceStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: tolerance is illegal, it must be a positive integer", toleranceStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "tolerance", toleranceStr,
				"it must be a positive integer")
		}
		if tolerance <= 0 {
			log.Errorf(ctx, "`%s`: duration is illegal, it must be a positive integer and not less than 0", toleranceStr)
			return pingParams, spec.ResponseFailWithFlags(spec.ParameterIllegal, "tolerance", toleranceStr,
				"it must be a positive integer and not less than 0")
		}
	} else {
		tolerance = 10
	}

	pingParams.tolerance = tolerance
	pingParams.expectedDelay = expectedDelay
	pingParams.expectedPercent = expectedPercent
	pingParams.duration = duration
	pingParams.quantity = quantity
	pingParams.destinationIp = destinationIp
	pingParams.experimentType = experimentType
	return pingParams, spec.Success()
}

func (npe *NetworkPingExecutor) SetChannel(channel spec.Channel) {
	npe.channel = channel
}

func (npe *NetworkPingExecutor) start(ctx context.Context, params *PingParams) *spec.Response {
	switch params.experimentType {
	case tc.Loss:
		return npe.onLoss(ctx, params)
	case tc.Corrupt:
		return npe.onCorrupt(ctx, params)
	case tc.Reorder:
		return npe.onReorder(ctx, params)
	case tc.Duplicate:
		return npe.onDuplicate(ctx, params)
	case tc.Delay:
		return npe.onDelay(ctx, params)
	default:
		return spec.ReturnFail(spec.ParameterIllegal, "UnSupported type")
	}
}

func (npe *NetworkPingExecutor) onDelay(ctx context.Context, params *PingParams) *spec.Response {
	interval, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(params.duration)/float64(params.quantity)), 64)
	response := npe.channel.Run(ctx, "ping", fmt.Sprintf(`%s -i %.2f -c %d -A | grep "icmp_seq" | awk -F '[ =]' '{print $10}'`,
		params.destinationIp, interval, params.quantity))
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, response.Err)
	}

	var delayCount int
	delayList := strings.Split(response.Result.(string), "\n")
	for _, delayStr := range delayList {
		if delay, err := strconv.Atoi(strings.TrimSpace(delayStr)); err == nil {
			delayCount += delay
		}
	}

	averageDelay := float64(delayCount) / float64(params.quantity)
	if checkPercent(averageDelay, params.expectedDelay, params.tolerance) {
		delayJson, _ := json.Marshal(map[string]float64{"averageDelay": averageDelay})
		return spec.ReturnSuccess(string(delayJson))
	} else {
		return spec.ResponseFailWithFlags(spec.PingSelfVerifyFailed, params.experimentType, fmt.Sprintf("%d", params.expectedDelay), fmt.Sprintf("%.2f", averageDelay))
	}
}

func (npe *NetworkPingExecutor) onDuplicate(ctx context.Context, params *PingParams) *spec.Response {
	interval, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(params.duration)/float64(params.quantity)), 64)
	response := npe.channel.Run(ctx, "ping", fmt.Sprintf(`%s -i %.2f -c %d -A | grep "icmp_seq" | grep "DUP!" | wc -l`, params.destinationIp, interval, params.quantity))
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, response.Err)
	}

	duplicate, _ := strconv.Atoi(strings.TrimSpace(response.Result.(string)))
	duplicatePercent := float64(duplicate) * 100 / float64(params.quantity)
	if checkPercent(duplicatePercent, params.expectedPercent, params.tolerance) {
		duplicateJson, _ := json.Marshal(map[string]float64{"duplicatePercent": duplicatePercent})
		return spec.ReturnSuccess(string(duplicateJson))
	} else {
		return spec.ResponseFailWithFlags(spec.PingSelfVerifyFailed, params.experimentType, fmt.Sprintf("%d", params.expectedPercent), fmt.Sprintf("%.2f", duplicatePercent))
	}
}

func (npe *NetworkPingExecutor) onReorder(ctx context.Context, params *PingParams) *spec.Response {
	interval, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(params.duration)/float64(params.quantity)), 64)
	response := npe.channel.Run(ctx, "ping", fmt.Sprintf(`%s -i %.2f -c %d -A | grep "icmp_seq" | awk -F '[ =]' '{print $6}'`,
		params.destinationIp, interval, params.quantity))
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, response.Err)
	}

	var reorderPacketCount int
	seqList := strings.Split(strings.TrimSpace(response.Result.(string)), "\n")

	var maxSeq int
	for _, seqStr := range seqList {
		if seq, err := strconv.Atoi(strings.TrimSpace(seqStr)); err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, err.Error())
		} else {
			if seq > maxSeq {
				maxSeq = seq
			} else {
				reorderPacketCount++
			}
		}
	}

	reorderPercent := float64(reorderPacketCount) * 100 / float64(params.quantity)
	if checkPercent(reorderPercent, params.expectedPercent, params.tolerance) {
		reorderJson, _ := json.Marshal(map[string]float64{"reorderPercent": reorderPercent})
		return spec.ReturnSuccess(string(reorderJson))
	} else {
		return spec.ResponseFailWithFlags(spec.PingSelfVerifyFailed, params.experimentType,
			fmt.Sprintf("%d", params.expectedPercent), fmt.Sprintf("%.2f", reorderPercent))
	}
}

func (npe *NetworkPingExecutor) onCorrupt(ctx context.Context, params *PingParams) *spec.Response {
	interval, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(params.duration)/float64(params.quantity)), 64)
	response := npe.channel.Run(ctx, "ping", fmt.Sprintf(`%s -i %.2f -c %d -A | grep "icmp_seq" | grep "%s" | wc -l`,
		params.destinationIp, interval, params.quantity, params.destinationIp))
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, response.Err)
	}

	if receivedPacketCount, err := strconv.Atoi(strings.TrimSpace(response.Result.(string))); err != nil {
		return spec.ReturnFail(spec.OsCmdExecFailed, err.Error())
	} else {
		corruptPercent := float64(params.quantity-receivedPacketCount) * 100 / float64(params.quantity)
		if checkPercent(corruptPercent, params.expectedPercent, params.tolerance) {
			corruptJson, _ := json.Marshal(map[string]float64{"corruptPercent": corruptPercent})
			return spec.ReturnSuccess(string(corruptJson))
		} else {
			return spec.ResponseFailWithFlags(spec.PingSelfVerifyFailed, params.experimentType,
				fmt.Sprintf("%d", params.expectedPercent), fmt.Sprintf("%.2f", corruptPercent))
		}
	}
}

func (npe *NetworkPingExecutor) onLoss(ctx context.Context, params *PingParams) *spec.Response {
	interval, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(params.duration)/float64(params.quantity)), 64)
	response := npe.channel.Run(ctx, "ping", fmt.Sprintf(`%s -i %.2f -c %d | grep "packet loss"`,
		params.destinationIp, interval, params.quantity))
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, response.Err)
	}

	reg := regexp.MustCompile(`,[^,]+,\s(.+)% packet loss`)
	lossStr := reg.FindAllStringSubmatch(response.Result.(string), -1)[0][1]
	if lossStr == "" {
		return spec.ReturnFail(spec.OsCmdExecFailed, "Failed to obtain packet loss rate by ping")
	}
	lossPercent, _ := strconv.ParseFloat(strings.TrimSpace(lossStr), 64)
	if checkPercent(lossPercent, params.expectedPercent, params.tolerance) {
		lossJson, _ := json.Marshal(map[string]float64{"lossPercent": lossPercent})
		return spec.ReturnSuccess(string(lossJson))
	} else {
		return spec.ResponseFailWithFlags(spec.PingSelfVerifyFailed, params.experimentType,
			fmt.Sprintf("%d", params.expectedPercent), fmt.Sprintf("%.2f", lossPercent))
	}
}
