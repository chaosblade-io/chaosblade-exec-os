package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/chaosblade-io/chaosblade-exec-os/version"
)

func main() {
	var (
		jsonOutput = flag.Bool("json", false, "Output version info in JSON format")
		short      = flag.Bool("short", false, "Output short version string only")
		full       = flag.Bool("full", false, "Output full version string")
	)
	flag.Parse()

	if *short {
		fmt.Println(version.GetVersion())
		return
	}

	if *full {
		fmt.Println(version.GetFullVersion())
		return
	}

	if *jsonOutput {
		info := version.GetVersionInfo()
		jsonData, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	// 默认输出
	info := version.GetVersionInfo()
	fmt.Printf("ChaosBlade Exec OS\n")
	fmt.Printf("==================\n")
	fmt.Printf("Version:     %s\n", info.Version)
	fmt.Printf("Git Commit:  %s\n", info.GitCommit)
	fmt.Printf("Build Time:  %s\n", info.BuildTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Go Version:  %s\n", info.GoVersion)
	fmt.Printf("Platform:    %s\n", info.Platform)
	fmt.Printf("Architecture: %s\n", info.Architecture)
	fmt.Printf("Is Release:  %t\n", version.IsRelease())
}
