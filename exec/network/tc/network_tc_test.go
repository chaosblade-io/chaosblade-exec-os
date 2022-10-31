package tc

import (
	"testing"
)

type buildtargetfilterparam = struct {
	input  struct {
		localPort string
		remotePort string
		destIpRules []string
		excludePorts []string
		excludeIpRules []string
		args string
		netInterface string
		protocol string
	}
	expect string
}
func TestbuildTargetFilterPortAndIp(t *testing.T) {
	var tests []buildtargetfilterparam
	var test1, test2 buildtargetfilterparam
	test1.input.remotePort = "6000"
	test1.input.destIpRules = append(test1.input.destIpRules, "match ip dst 10.18.2.156")
	test1.input.args = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms"
	test1.input.netInterface = "ens33"
	test1.input.protocol = "6"
	test1.expect = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms && \\\n\t\t\t\t\t\ttc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 6000 0xffff  \\\n                                         match ip protocol 6 0xff flowid 1:4"

	test2.input.localPort = "9100"
	test2.input.remotePort = "6000"
	test2.input.destIpRules = append(test2.input.destIpRules, "match ip dst 10.18.2.50")
	test2.input.excludePorts = append(test2.input.excludePorts, "13579")
	test2.input.excludeIpRules = append(test2.input.excludeIpRules, "match ip dst 10.18.1.138")
	test2.input.protocol = "17"
	test2.input.args = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms"
	test2.input.netInterface = "ens33"
	test2.expect = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms && \\\n\t\t\t\t\t\ttc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip sport 9100 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n\t\t\t\t\t\ttc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip dport 6000 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n\t\t\t\ttc filter add dev ens33 parent 1: prio 3 protocol ip u32 match ip dst 10.18.1.138  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n\t\t\t\ttc filter add dev ens33 parent 1: prio 3 protocol ip u32 match ip dport 13579 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n\t\t\t\ttc filter add dev ens33 parent 1: prio 3 protocol ip u32 match ip sport 13579 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3"

	for _, tt := range tests {
		//localPort, remotePort string, destIpRules, excludePorts, excludeIpRules []string, args string, netInterface, protocol string
		returnargs := buildTargetFilterPortAndIp(tt.input.localPort, tt.input.remotePort, tt.input.destIpRules, tt.input.excludePorts, tt.input.excludeIpRules, tt.input.args, tt.input.netInterface, tt.input.protocol)
		if returnargs != tt.expect {
			t.Errorf("unexpected result: %s, expected: %s", returnargs, tt.expect)
		}
	}
}
