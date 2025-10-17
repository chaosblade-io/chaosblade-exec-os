//go:build linux

package mem

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/shirou/gopsutil/mem"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

// getAvailableAndTotalV2 获取 cgroup v2 环境下的可用和总内存
func getAvailableAndTotalV2(ctx context.Context, burnMemMode string, includeBufferCache bool) (int64, int64, error) {
	pid := ctx.Value(channel.NSTargetFlagName)
	total := int64(0)
	available := int64(0)

	if pid != nil {
		p, err := strconv.Atoi(pid.(string))
		if err != nil {
			return 0, 0, fmt.Errorf("load cgroup error, %v", err)
		}

		cgroupRoot := ctx.Value("cgroup-root")
		if cgroupRoot == "" {
			cgroupRoot = "/sys/fs/cgroup"
		}

		log.Debugf(ctx, "get mem usage by cgroup v2, root path: %s", cgroupRoot)

		// 查找 cgroup v2 路径
		cgroupPath, err := cgroups.FindCGroupV2Path(ctx, strconv.Itoa(p), cgroupRoot.(string))
		if err != nil {
			log.Errorf(ctx, "failed to find cgroup v2 path: %v", err)
			return getSystemMemory(burnMemMode, includeBufferCache)
		}

		if cgroupPath == "" {
			log.Warnf(ctx, "no cgroup v2 path found, falling back to system memory")
			return getSystemMemory(burnMemMode, includeBufferCache)
		}

		// 创建 CGroupV2Impl 实例并获取内存限制
		cg := cgroups.NewCGroupV2Impl(cgroupPath)
		limit, defined, err := cg.MemoryLimit()
		if err != nil {
			log.Errorf(ctx, "failed to get cgroup v2 memory limit: %v", err)
			return getSystemMemory(burnMemMode, includeBufferCache)
		}

		if !defined || limit == 0 {
			log.Warnf(ctx, "cgroup v2 memory limit not defined or unlimited, falling back to system memory")
			return getSystemMemory(burnMemMode, includeBufferCache)
		}

		// 获取当前内存使用情况
		usage, err := getCGroupV2MemoryUsage(ctx, cgroupPath)
		if err != nil {
			log.Errorf(ctx, "failed to get cgroup v2 memory usage: %v", err)
			return getSystemMemory(burnMemMode, includeBufferCache)
		}

		total = limit
		available = total - usage
		if burnMemMode == "ram" && !includeBufferCache {
			// 对于 cgroup v2，我们需要从 memory.stat 文件中获取缓存信息
			cache, err := getCGroupV2MemoryCache(ctx, cgroupPath)
			if err == nil {
				available += cache
			}
		}

		log.Infof(ctx, "cgroup v2 memory: total=%d, usage=%d, available=%d", total, usage, available)
		return total, available, nil
	}

	return getSystemMemory(burnMemMode, includeBufferCache)
}

// getCGroupV2MemoryUsage 获取 cgroup v2 的内存使用量
func getCGroupV2MemoryUsage(ctx context.Context, cgroupPath string) (int64, error) {
	// 读取 memory.current 文件
	currentFile := cgroupPath + "/memory.current"
	content, err := os.ReadFile(currentFile)
	if err != nil {
		return 0, err
	}

	usage, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, err
	}

	return usage, nil
}

// getCGroupV2MemoryCache 获取 cgroup v2 的缓存使用量
func getCGroupV2MemoryCache(ctx context.Context, cgroupPath string) (int64, error) {
	// 读取 memory.stat 文件并解析缓存信息
	statFile := cgroupPath + "/memory.stat"
	content, err := os.ReadFile(statFile)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	var cache int64
	for _, line := range lines {
		if strings.HasPrefix(line, "file ") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				if val, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					cache += val
				}
			}
		}
	}

	return cache, nil
}

// getSystemMemory 获取系统内存信息（回退方案）
func getSystemMemory(burnMemMode string, includeBufferCache bool) (int64, int64, error) {
	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, err
	}
	total := int64(virtualMemory.Total)
	available := int64(virtualMemory.Free)
	if burnMemMode == "ram" && !includeBufferCache {
		available = available + int64(virtualMemory.Buffers+virtualMemory.Cached)
	}
	return total, available, nil
}
