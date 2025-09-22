package tc

import (
	"strings"
	"testing"
)

// Test our exclude IP and port fixes
func TestExcludeIPAndPortFixes(t *testing.T) {
	
	// Test 1: Test getExcludeIpRules function
	t.Run("getExcludeIpRules function", func(t *testing.T) {
		testExcludeIp := "10.38.160.189"
		excludeIpRules := getExcludeIpRules(testExcludeIp)
		
		// Should generate both src and dst rules
		expected := []string{
			"match ip src 10.38.160.189",
			"match ip dst 10.38.160.189",
		}
		
		if len(excludeIpRules) != 2 {
			t.Errorf("Expected 2 rules, got %d", len(excludeIpRules))
			return
		}
		
		for i, expectedRule := range expected {
			if i >= len(excludeIpRules) || excludeIpRules[i] != expectedRule {
				t.Errorf("Expected rule %d to be '%s', got '%s'", i, expectedRule, excludeIpRules[i])
				return
			}
		}
		t.Log("✅ getExcludeIpRules correctly generates src and dst rules")
	})
	
	// Test 2: Test buildExcludeFilterToNewBand priority
	t.Run("buildExcludeFilterToNewBand priority", func(t *testing.T) {
		netInterface := "eth0"
		excludePortRanges := [][]int{{47214, 47214}}
		excludeIp := "10.38.160.189"
		
		args := buildExcludeFilterToNewBand(netInterface, excludePortRanges, excludeIp)
		
		// Check if priority is set to 1 (highest)
		if !strings.Contains(args, "prio 1") {
			t.Errorf("Expected 'prio 1' in generated commands, got: %s", args)
			return
		}
		
		// Check if both src and dst IP rules are present
		if !strings.Contains(args, "match ip src 10.38.160.189") {
			t.Errorf("Expected 'match ip src 10.38.160.189' in generated commands")
			return
		}
		
		if !strings.Contains(args, "match ip dst 10.38.160.189") {
			t.Errorf("Expected 'match ip dst 10.38.160.189' in generated commands")
			return
		}
		
		// Check if port rules are present with priority 1
		if !strings.Contains(args, "match ip dport 47214") || !strings.Contains(args, "match ip sport 47214") {
			t.Errorf("Expected port 47214 rules in generated commands")
			return
		}
		
		t.Log("✅ buildExcludeFilterToNewBand correctly uses priority 1 and includes src/dst IP rules")
	})
	
	// Test 3: Test the scenario from the bug report
	t.Run("Bug report scenario", func(t *testing.T) {
		netInterface := "eth0"
		excludePortRanges := [][]int{{47214, 47214}}
		excludeIp := "10.38.160.189"
		
		// This would be the path taken in startNet() when only excludePort and excludeIp are specified
		classRule := "netem loss 100%"
		args := buildNetemToDefaultBandsArgs(netInterface, classRule)
		excludeFilters := buildExcludeFilterToNewBand(netInterface, excludePortRanges, excludeIp)
		finalCommand := args + excludeFilters
		
		// Verify the command structure
		expectedComponents := []string{
			"qdisc add dev eth0 parent 1:1 netem loss 100%",  // Apply loss to band 1
			"qdisc add dev eth0 parent 1:2 netem loss 100%",  // Apply loss to band 2  
			"qdisc add dev eth0 parent 1:3 netem loss 100%",  // Apply loss to band 3
			"qdisc add dev eth0 parent 1:4 handle 40: prio",  // Band 4 for excluded traffic
			"prio 1",                                          // High priority for excludes
			"match ip src 10.38.160.189",                     // Source IP exclusion
			"match ip dst 10.38.160.189",                     // Dest IP exclusion
			"flowid 1:4",                                      // Route to safe band
		}
		
		for _, component := range expectedComponents {
			if !strings.Contains(finalCommand, component) {
				t.Errorf("Missing component: %s in command: %s", component, finalCommand)
				return
			}
		}
		
		t.Log("✅ Bug report scenario generates correct tc rules")
	})
	
	// Test 4: Test multiple IPs
	t.Run("Multiple exclude IPs", func(t *testing.T) {
		testExcludeIp := "10.38.160.189,192.168.1.100"
		excludeIpRules := getExcludeIpRules(testExcludeIp)
		
		// Should generate both src and dst rules for each IP
		expected := []string{
			"match ip src 10.38.160.189",
			"match ip dst 10.38.160.189",
			"match ip src 192.168.1.100",
			"match ip dst 192.168.1.100",
		}
		
		if len(excludeIpRules) != 4 {
			t.Errorf("Expected 4 rules for 2 IPs, got %d", len(excludeIpRules))
			return
		}
		
		for i, expectedRule := range expected {
			if i >= len(excludeIpRules) || excludeIpRules[i] != expectedRule {
				t.Errorf("Expected rule %d to be '%s', got '%s'", i, expectedRule, excludeIpRules[i])
				return
			}
		}
		t.Log("✅ Multiple exclude IPs correctly handled")
	})
}