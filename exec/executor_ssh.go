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
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/version"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"golang.org/x/crypto/ssh"

	"github.com/howeyc/gopass"
)

const (
	DefaultInstallPath = "/opt/chaosblade"
	BladeBin           = "blade"
	DefaultSSHPort     = 22
	BladeReleaseURL    = "https://chaosblade.oss-cn-hangzhou.aliyuncs.com/agent/github/%s/chaosblade-%s-linux-amd64.tar.gz"
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

var SSHKeyFlag = &spec.ExpFlag{
	Name:     "ssh-key",
	Desc:     "Use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

var SSHKeyPassphraseFlag = &spec.ExpFlag{
	Name:     "ssh-key-passphrase",
	Desc:     "Use this flag when the channel is ssh",
	NoArgs:   true,
	Required: false,
}

var BladeRelease = &spec.ExpFlag{
	Name:     "blade-release",
	Desc:     "Blade release package，use this flag when the channel is ssh",
	NoArgs:   false,
	Required: false,
}

var OverrideBladeRelease = &spec.ExpFlag{
	Name:     "override-blade-release",
	Desc:     "Override blade release，use this flag when the channel is ssh",
	NoArgs:   true,
	Required: false,
}

var InstallPath = &spec.ExpFlag{
	Name:     "install-path",
	Desc:     "install path default /opt/chaosblade，use this flag when the channel is ssh",
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
	key := expModel.ActionFlags[SSHKeyFlag.Name]
	port := DefaultSSHPort
	portStr := expModel.ActionFlags[SSHPortFlag.Name]
	var err error
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil || port < 1 {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "port", port, "it must be a positive integer")
		}
	}

	var client *SSHClient
	var password []byte
	var keyPassphrase []byte

	if key == "" {
		fmt.Print("Please enter password:")
		password, err = gopass.GetPasswd()
		if err != nil {
			log.Errorf(ctx, "password is illegal, err: %s", err.Error())
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "password", "****", err.Error())
		}
	} else {
		useKeyPassphrase := expModel.ActionFlags[SSHKeyPassphraseFlag.Name] == "true"
		if useKeyPassphrase {
			fmt.Print(fmt.Sprintf("Please Enter passphrase for key '%s':", key))
			keyPassphrase, err = gopass.GetPasswd()
			if err != nil {
				log.Errorf(ctx, "`%s`: get passphrase failed, err: %s", key, err.Error())
				return spec.ResponseFailWithFlags(spec.ParameterIllegal, "passphrase", key, err.Error())
			}
		}
	}

	client = &SSHClient{
		Host:          expModel.ActionFlags[SSHHostFlag.Name],
		Username:      expModel.ActionFlags[SSHUserFlag.Name],
		Key:           expModel.ActionFlags[SSHKeyFlag.Name],
		keyPassphrase: strings.Replace(string(keyPassphrase), "\n", "", -1),
		Password:      strings.Replace(string(password), "\n", "", -1),
		Port:          port,
	}

	matchers := spec.ConvertExpMatchersToString(expModel, func() map[string]spec.Empty {
		return excludeSSHFlags()
	})
	installPath := expModel.ActionFlags[InstallPath.Name]
	if installPath == "" {
		installPath = DefaultInstallPath
	}
	bladeBin := path.Join(installPath, BladeBin)

	if _, ok := spec.IsDestroy(ctx); ok {
		output, err := client.RunCommand(fmt.Sprintf("%s destroy %s", bladeBin, uid))
		return ConvertOutputToResponse(ctx, string(output), err, nil)
	} else {
		overrideBladeRelease := expModel.ActionFlags[OverrideBladeRelease.Name] == "true"
		if overrideBladeRelease {
			if resp, ok := client.RunCommandWithResponse(ctx, fmt.Sprintf(`rm -rf %s`, installPath)); !ok {
				return resp
			}
		}

		if resp, ok := client.RunCommandWithResponse(ctx, fmt.Sprintf(`if [ ! -d "%s" ]; then mkdir %s; fi;`, installPath, installPath)); !ok {
			return resp
		}

		bladeReleaseURL := expModel.ActionFlags[BladeRelease.Name]
		if bladeReleaseURL == "" {
			bladeReleaseURL = fmt.Sprintf(BladeReleaseURL, version.BladeVersion, version.BladeVersion)
		}
		installCommand :=
			fmt.Sprintf(`if  [ ! -f "%s" ];then
														wget %s;
														if [ $? -ne 0 ]; then exit 1; fi;
														tar -zxf $(echo "%s" |awk -F '/' '{print $NF}') -C %s --strip-components 1;
														if [ $? -ne 0 ]; then exit 1; fi;
														rm -f $(echo "%s" |awk -F '/' '{print $NF}');
													fi`, bladeBin, bladeReleaseURL, bladeReleaseURL, installPath, bladeReleaseURL)
		if resp, ok := client.RunCommandWithResponse(ctx, installCommand); !ok {
			return resp
		}
		createCommand := fmt.Sprintf("%s create %s %s %s --uid %s -d", bladeBin, expModel.Target, expModel.ActionName, matchers, uid)
		output, err := client.RunCommand(createCommand)
		log.Debugf(ctx, "exec blade create command: %s, result: %s, err %s", createCommand, string(output), err)
		return ConvertOutputToResponse(ctx, string(output), err, nil)
	}
}

type SSHClient struct {
	Host          string
	Username      string
	Key           string
	keyPassphrase string
	Password      string
	Port          int
	client        *ssh.Client
	cipherList    []string
}

func (c SSHClient) RunCommandWithResponse(ctx context.Context, cmd string) (*spec.Response, bool) {
	buf, err := c.RunCommand(cmd)
	if err != nil {
		log.Errorf(ctx, spec.OsCmdExecFailed.Sprintf(cmd, err))
		if buf != nil {
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, cmd, fmt.Sprintf("buf is %s, %v", string(buf), err)), false
		}
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, cmd, err), false
	}
	return nil, true
}

func (c SSHClient) RunCommand(command string) ([]byte, error) {
	if c.client == nil {
		if err := c.connect(); err != nil {
			return nil, err
		}
	}
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	buf, err := session.CombinedOutput(command)
	return buf, err
}

func ConvertOutputToResponse(ctx context.Context, output string, err error, defaultResponse *spec.Response) *spec.Response {
	if err != nil {
		response := spec.Decode(err.Error(), defaultResponse)
		if response.Success {
			return response
		}
		output = strings.TrimSpace(output)
		log.Errorf(ctx, spec.SshExecFailed.Sprintf(output, err))
		return spec.ResponseFailWithFlags(spec.SshExecFailed, output, err)
	}
	output = strings.TrimSpace(output)
	if output == "" {
		log.Errorf(ctx, spec.SshExecNothing.Msg)
		return spec.ResponseFailWithFlags(spec.SshExecNothing)
	}
	response := spec.Decode(output, defaultResponse)
	return response
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

	auth := make([]ssh.AuthMethod, 0)
	if c.Key == "" {
		auth = append(auth, ssh.Password(c.Password))
	} else {
		pemBytes, err := os.ReadFile(c.Key)
		if err != nil {
			return err
		}

		var signer ssh.Signer
		if c.keyPassphrase == "" {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(c.keyPassphrase))
		}
		if err != nil {
			return err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}

	clientConfig := ssh.ClientConfig{
		User:   c.Username,
		Config: config,
		Auth:   auth,
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
	flags[SSHKeyFlag.Name] = spec.Empty{}
	flags[SSHKeyPassphraseFlag.Name] = spec.Empty{}
	flags[BladeRelease.Name] = spec.Empty{}
	flags[OverrideBladeRelease.Name] = spec.Empty{}
	flags[InstallPath.Name] = spec.Empty{}
	return flags
}
