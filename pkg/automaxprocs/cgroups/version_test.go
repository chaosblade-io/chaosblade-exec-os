//go:build linux

package cgroups

import (
	"context"
	"testing"
)

func TestDetectCGroupVersion(t *testing.T) {
	ctx := context.Background()

	// 测试默认路径
	version := DetectCGroupVersion(ctx, "")
	if version != CGroupV1 && version != CGroupV2 {
		t.Errorf("Expected CGroupV1 or CGroupV2, got %v", version)
	}

	// 测试指定路径
	version = DetectCGroupVersion(ctx, "/sys/fs/cgroup")
	if version != CGroupV1 && version != CGroupV2 {
		t.Errorf("Expected CGroupV1 or CGroupV2, got %v", version)
	}
}

func TestIsCGroupV2(t *testing.T) {
	ctx := context.Background()

	// 测试默认路径
	isV2 := IsCGroupV2(ctx, "")
	version := DetectCGroupVersion(ctx, "")
	expected := (version == CGroupV2)

	if isV2 != expected {
		t.Errorf("IsCGroupV2() = %v, expected %v", isV2, expected)
	}
}
