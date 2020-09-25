/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package exec

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/version"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/howeyc/gopass"
)

const (
	BladeBin        = "/opt/chaosblade/blade"
	DefaultSSHPort  = 22
	BladeReleaseURL = "https://chaosblade.oss-cn-hangzhou.aliyuncs.com/agent/github/%s/chaosblade-%s-linux-amd64.tar.gz"
)

// support ssh channel flags
var ChannelFlag = &spec.ExpFlag{
	Name:     "channel",
	Desc:     "Select the channel for execution, and you can now select SSH",
	NoArgs:   false,
	Required: false,
}

var SSHHostFlag = &spec.ExpFlag{
	Name:     "ssh-host",
	Desc:     "Use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

var SSHUserFlag = &spec.ExpFlag{
	Name:     "ssh-user",
	Desc:     "Use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

var SSHPortFlag = &spec.ExpFlag{
	Name:     "ssh-port",
	Desc:     "Use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

var BladeRelease = &spec.ExpFlag{
	Name:     "blade-release",
	Desc:     "Blade release package，use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

type SSHExecutor struct {
	spec.Executor
}

func NewSSHExecutor() spec.Executor {
	return &SSHExecutor{}
}

func (*SSHExecutor) Name() string {
	return "ssh"
}

func (e *SSHExecutor) SetChannel(channel spec.Channel) {
}

func (e *SSHExecutor) Exec(uid string, ctx context.Context, expModel *spec.ExpModel) *spec.Response {
	fmt.Print("Please enter password:")
	password, err := gopass.GetPasswd()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
	}

	port := DefaultSSHPort
	portStr := expModel.ActionFlags[SSHPortFlag.Name]
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil || port < 1 {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], "--port value must be a positive integer")
		}
	}

	client := &SSHClient{
		Host:     expModel.ActionFlags[SSHHostFlag.Name],
		Username: expModel.ActionFlags[SSHUserFlag.Name],
		Password: strings.Replace(string(password), "\n", "", -1),
		Port:     port,
	}

	matchers := spec.ConvertExpMatchersToString(expModel, func() map[string]spec.Empty {
		return excludeSSHFlags()
	})

	if _, ok := spec.IsDestroy(ctx); ok {
		str, err := client.Run(fmt.Sprintf("%s destroy %s", BladeBin, uid))
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
		}
		return spec.Decode(str, nil)
	} else {
		bladeReleaseURL := expModel.ActionFlags[BladeRelease.Name]
		if bladeReleaseURL == "" {
			bladeReleaseURL = fmt.Sprintf(BladeReleaseURL, version.BladeVersion, version.BladeVersion)
		}
		assembly :=
			fmt.Sprintf(`if  [ ! -f "/opt/chaosblade/blade" ];then
														if  [ ! -d "/opt" ];then
															mkdir /opt
														fi
														wget %s
														tar -zxf $(echo "%s" |awk -F '/' '{print $NF}') -C /opt
														mv /opt/$(tar tf $(echo "%s" |awk -F '/' '{print $NF}') | head -1 | cut -f1 -d/) /opt/chaosblade
													fi`, bladeReleaseURL, bladeReleaseURL, bladeReleaseURL)
		_, err := client.Run(assembly)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
		}

		execute := fmt.Sprintf("%s create %s %s %s --uid %s -d", BladeBin, expModel.Target, expModel.ActionName, matchers, uid)
		output, err := client.Run(execute)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
		}
		return spec.Decode(output, nil)
	}

}

type SSHClient struct {
	Host       string
	Username   string
	Password   string
	Port       int
	client     *ssh.Client
	cipherList []string
}

func (c SSHClient) Run(shell string) (string, error) {
	if c.client == nil {
		if err := c.connect(); err != nil {
			return "", err
		}
	}
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	buf, err := session.CombinedOutput(shell)
	return string(buf), err
}

func (c *SSHClient) connect() error {

	var config ssh.Config
	if len(c.cipherList) == 0 {
		config = ssh.Config{
			Ciphers: []string{"aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com", "arcfour256", "arcfour128", "aes128-cbc", "3des-cbc", "aes192-cbc", "aes256-cbc"},
		}
	} else {
		config = ssh.Config{
			Ciphers: c.cipherList,
		}
	}

	clientConfig := ssh.ClientConfig{
		User:   c.Username,
		Config: config,
		Auth:   []ssh.AuthMethod{ssh.Password(c.Password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 10 * time.Second,
	}
	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port), &clientConfig)
	if err != nil {
		return err
	}
	c.client = sshClient
	return nil
}

func excludeSSHFlags() map[string]spec.Empty {
	flags := make(map[string]spec.Empty, 0)
	flags[ChannelFlag.Name] = spec.Empty{}
	flags[SSHHostFlag.Name] = spec.Empty{}
	flags[SSHUserFlag.Name] = spec.Empty{}
	flags[SSHPortFlag.Name] = spec.Empty{}
	flags[BladeRelease.Name] = spec.Empty{}
	return flags
}
