package cgroups

import "testing"

func Test_replaceCgroupFsPathForDaemonSetPod(t *testing.T) {
	type args struct {
		mountPointPath string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "pass",
			args: args{
				mountPointPath: "/sys/fs/cgroup/cpu",
			},
			want: "/host-sys/fs/cgroup/cpu",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceCgroupFsPathForDaemonSetPod(tt.args.mountPointPath, "/host-sys/fs/cgroup/"); got != tt.want {
				t.Errorf("replaceCgroupFsPathForDaemonSetPod() = %v, want %v", got, tt.want)
			}
		})
	}
}
