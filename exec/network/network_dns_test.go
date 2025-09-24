package network

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/goodhosts/hostsfile"
)

func Test_replaceApplier_e2e(t *testing.T) {
	type args struct {
		domainArgs  string
		ip          string
		uid         string
		originHosts string
	}
	pre := func(originHosts string) (*os.File, error) {
		temp, err := os.CreateTemp("", "test-hosts-")
		if err != nil {
			t.Errorf("create temp file error: %v", err)
			return nil, err
		}
		if err := os.WriteFile(temp.Name(), []byte(originHosts), 0o644); err != nil {
			t.Errorf("write temp file error: %v", err)
			return nil, err
		}
		return temp, nil
	}
	tests := []struct {
		pre        func(string) (*os.File, error)
		name       string
		args       args
		successful bool
	}{
		{
			pre:  pre,
			name: "pass",
			args: args{
				originHosts: `
127.0.0.1	localhost
192.168.1.1 foo.bar
192.168.1.2 bar.baz
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "127.0.0.1",
				uid:        "foo",
			},
			successful: true,
		},
		{
			pre:  pre,
			name: "one exists, one not exists",
			args: args{
				originHosts: `
127.0.0.1	localhost
192.168.1.1 foo.bar
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "127.0.0.1",
				uid:        "foo",
			},
			successful: true,
		},
		{
			pre:  pre,
			name: "not exists",
			args: args{
				originHosts: `
127.0.0.1	localhost
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "127.0.0.1",
				uid:        "foo",
			},
			successful: true,
		},
		{
			pre:  pre,
			name: "already exists",
			args: args{
				originHosts: `
127.0.0.1	localhost
127.0.0.1 foo.bar
127.0.0.1 bar.baz
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "127.0.0.1",
				uid:        "foo",
			},
			successful: true,
		},
		{
			pre:  pre,
			name: "invalid ip",
			args: args{
				originHosts: `
127.0.0.1	localhost
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar",
				ip:         "not-an-ip",
				uid:        "foo",
			},
			successful: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pre != nil {
				temp, err := tt.pre(tt.args.originHosts)
				if err != nil {
					t.Errorf("pre() error: %v", err)
					return
				}
				defer func() {
					_ = os.Remove(temp.Name())
				}()
				hosts = temp.Name()
			}
			r := &replaceApplier{
				ch: channel.NewLocalChannel(),
			}
			if got := r.Start(
				context.Background(),
				tt.args.uid,
				tt.args.domainArgs,
				tt.args.ip,
			); got.Success != tt.successful {
				t.Errorf("start() = %v, want %v", got, tt.successful)
				return
			} else {
				t.Logf("start() = %v, want %v", got, tt.successful)
			}

			if !tt.successful {
				return
			}

			hosts, err := hostsfile.NewCustomHosts(hosts)
			if err != nil {
				t.Errorf("create hosts file error: %v", err)
				return
			}
			for _, domain := range strings.Split(tt.args.domainArgs, sep) {
				if ok := hosts.Has(tt.args.ip, domain); !ok {
					t.Errorf("start() failed, domain %s not found in hosts file", domain)
				}
				// hostsfile/hosts#Add will remove hosts from other ips if it already exists
				if ok := hosts.HasIP("192.168.1.1"); ok {
					t.Errorf("start() failed, 192.168.1.1 should not be found in hosts file")
				}
				if ok := hosts.HasIP("192.168.1.2"); ok {
					t.Errorf("start() failed, 192.168.1.2 should not be found in hosts file")
				}
			}
			t.Log(hosts.String())

			e := NetworkDnsExecutor{
				channel: channel.NewLocalChannel(),
			}

			if got := e.stop(context.Background(), tt.args.uid); !got.Success {
				t.Errorf("stop() = %v, want %v", got, true)
			} else {
				t.Logf("stop() = %v, want %v", got, true)
			}
		})
	}
}

func Test_defaultApplier_e2e(t *testing.T) {
	type args struct {
		domainArgs  string
		ip          string
		uid         string
		originHosts string
	}
	pre := func(originHosts string) (*os.File, error) {
		temp, err := os.CreateTemp("", "test-hosts-")
		if err != nil {
			t.Errorf("create temp file error: %v", err)
			return nil, err
		}
		if err := os.WriteFile(temp.Name(), []byte(originHosts), 0o644); err != nil {
			t.Errorf("write temp file error: %v", err)
			return nil, err
		}
		return temp, nil
	}
	tests := []struct {
		pre        func(string) (*os.File, error)
		name       string
		args       args
		successful bool
	}{
		{
			pre:  pre,
			name: "pass",
			args: args{
				originHosts: `
127.0.0.1	localhost
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "192.168.1.1",
				uid:        "foo",
			},
			successful: true,
		},
		{
			pre:  pre,
			name: "already exists",
			args: args{
				originHosts: `
127.0.0.1	localhost
192.168.1.1 foo.bar bar.baz #chaosblade
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar,bar.baz",
				ip:         "192.168.1.1",
				uid:        "foo",
			},
			successful: false,
		},
		{
			pre:  pre,
			name: "empty domain",
			args: args{
				originHosts: `
127.0.0.1	localhost
8.8.8.8 my.domain
`,
				domainArgs: "",
				ip:         "127.0.0.1",
				uid:        "foo",
			},
			successful: true, // echo "" >> file is a valid command
		},
		{
			pre:  pre,
			name: "empty ip",
			args: args{
				originHosts: `
127.0.0.1	localhost
8.8.8.8 my.domain
`,
				domainArgs: "foo.bar",
				ip:         "",
				uid:        "foo",
			},
			successful: true, // echo " foo.bar #chaosblade" >> file is a valid command
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pre != nil {
				temp, err := tt.pre(tt.args.originHosts)
				if err != nil {
					t.Errorf("pre() error: %v", err)
					return
				}
				defer func() {
					_ = os.Remove(temp.Name())
				}()
				hosts = temp.Name()
			}
			r := &defaultApplier{
				ch: channel.NewLocalChannel(),
			}
			if got := r.Start(
				context.Background(),
				tt.args.uid,
				tt.args.domainArgs,
				tt.args.ip,
			); got.Success != tt.successful {
				t.Errorf("start() = %v, want %v", got, tt.successful)
				return
			} else {
				t.Logf("start() = %v, want %v", got, tt.successful)
			}

			if !tt.successful {
				return
			}

			h, err := hostsfile.NewCustomHosts(hosts)
			if err != nil {
				t.Errorf("create hosts file error: %v", err)
				return
			}
			for _, domain := range strings.Split(tt.args.domainArgs, sep) {
				if ok := h.Has(tt.args.ip, domain); !ok && tt.args.ip != "" && tt.args.domainArgs != "" {
					t.Errorf("start() failed, domain %s not found in hosts file", domain)
				}
			}
			t.Log(h.String())
		})
	}
}
