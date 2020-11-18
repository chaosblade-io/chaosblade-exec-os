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
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"golang.org/x/crypto/ssh"

	"github.com/howeyc/gopass"
)

const (
	DefaultInstallPath = "/opt/chaosblade"
	BladeBin           = "blade"
	DefaultSSHPort     = 22
	BladeReleaseURL    = "http://nginxfile.dds-sysu.tech:8080/%s/chaosblade-%s.tar.gz"
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
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], "--port value must be a positive integer")
		}
	}

	var client *SSHClient
	var password []byte
	var keyPassphrase []byte

	if key == "" {
		fmt.Print("Please enter password:")
		password, err = gopass.GetPasswd()
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
		}
	} else {
		useKeyPassphrase := expModel.ActionFlags[SSHKeyPassphraseFlag.Name] == "true"
		if useKeyPassphrase {
			fmt.Print(fmt.Sprintf("Please Enter passphrase for key '%s':", key))
			keyPassphrase, err = gopass.GetPasswd()
			if err != nil {
				return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
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
		return ConvertOutputToResponse(string(output), err, nil)
	} else {
		overrideBladeRelease := expModel.ActionFlags[OverrideBladeRelease.Name] == "true"
		if overrideBladeRelease {
			buf, err := client.RunCommand(fmt.Sprintf(`rm -rf %s`, installPath))
			if err != nil {
				if buf != nil {
					return spec.ReturnFail(spec.Code[spec.ExecCommandError], string(buf))
				}
				return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
			}
		}

		buf, err := client.RunCommand(fmt.Sprintf(`if [ ! -d "%s" ]; then mkdir %s; fi;`, installPath, installPath))
		if err != nil {
			if buf != nil {
				return spec.ReturnFail(spec.Code[spec.ExecCommandError], string(buf))
			}
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
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
		buf, err = client.RunCommand(installCommand)
		logrus.Debugf("exec command: %s, result: %s, err %s", installCommand, string(buf), err)
		if err != nil {
			if buf != nil {
				return spec.ReturnFail(spec.Code[spec.ExecCommandError], string(buf))
			}
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
		}
		createCommand := fmt.Sprintf("%s create %s %s %s --uid %s -d", bladeBin, expModel.Target, expModel.ActionName, matchers, uid)
		output, err := client.RunCommand(createCommand)
		logrus.Debugf("exec blade create command: %s, result: %s, err %s", createCommand, string(output), err)
		return ConvertOutputToResponse(string(output), err, nil)
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

func ConvertOutputToResponse(output string, err error, defaultResponse *spec.Response) *spec.Response {
	if err != nil {
		response := spec.Decode(err.Error(), defaultResponse)
		if response.Success {
			return response
		}
		output = strings.TrimSpace(output)
		if output != "" {
			return spec.ReturnFail(spec.Code[spec.ExecCommandError], fmt.Sprintf("result: %s, error: %s", output, err.Error()))
		}
		return spec.ReturnFail(spec.Code[spec.ExecCommandError], err.Error())
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return spec.ReturnFail(spec.Code[spec.ExecCommandError],
			"cannot get result message from remote host, please execute recovery and try again")
	}
	response := spec.Decode(output, defaultResponse)
	if response.Success {
		return response
	}
	if response.Code == spec.Code[spec.DecodeError].Code {
		return spec.ReturnFail(spec.Code[spec.ExecCommandError], output)
	}
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
		pemBytes, err := ioutil.ReadFile(c.Key)
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
