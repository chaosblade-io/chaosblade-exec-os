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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	arguments := &Args{}
	flag.StringVar(&arguments.packages, "p", "", "Golang compile package in chaosblade-exec-os.")
	flag.StringVar(&arguments.command, "c", "go", "Golang compile command.")
	flag.StringVar(&arguments.flags, "ldflags", "-linkmode external -extldflags -s -w", "Golang compile flags.")
	flag.StringVar(&arguments.target, "o", "", "Golang compile output dir.")
	flag.StringVar(&arguments.name, "n", "", "Golang compile output name.")
	flag.StringVar(&arguments.environ, "e", "", "Golang compile environment.")
	flag.Parse()

	if err := buildChain(generator, writer, compiler).build(arguments); nil != err {
		fmt.Println(err.Error())
	}
}

const mainGo = `
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

package main

import (
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
${- if .imports }
    ${- with .imports }
    ${- range $idx, $import := .}
	_ "${ $import | ToString }"
    ${- end }
    ${- end }
${- end }
)

func main() {
	model.Load("${.name}").Exec()
}
`

type Args struct {
	name     string
	packages string
	command  string
	flags    string
	target   string
	environ  string
	source   []byte
	home     string
}

type Chain struct {
	cursor int
	phases []func(chain *Chain, args *Args) error
}

func buildChain(phases ...func(chain *Chain, args *Args) error) *Chain {
	chain := &Chain{}
	chain.phases = append(append(chain.phases, func(chain *Chain, args *Args) error {
		workspace, _ := filepath.Abs(".")
		args.home = filepath.Join(workspace, "build", "main")
		if "" != args.target {
			args.target = filepath.Join(workspace, args.target)
		}
		if "" == args.target {
			args.target = filepath.Join(workspace, "target", args.name)
		}
		if "" == args.name {
			args.name = args.target[strings.LastIndex(args.target, string(filepath.Separator))+1:]
		}
		if "" == args.packages {
			args.packages = fmt.Sprintf("github.com/chaosblade-io/chaosblade-exec-os/%s", filepath.Dir(os.Args[len(os.Args)-1]))
		}
		return chain.build(args)
	}), phases...)
	return chain
}

func (that *Chain) build(args *Args) error {
	that.cursor = (that.cursor | 4) + 1
	if len(that.phases) <= that.cursor-5 {
		return nil
	}
	if err := that.phases[that.cursor-5](that, args); nil != err {
		return err
	}
	return nil
}

func generator(chain *Chain, args *Args) error {
	render, err := template.New("").
		Delims("${", "}").
		Funcs(map[string]interface{}{"ToString": func(v string) string {
			if "" == strings.TrimSpace(v) {
				return "fmt"
			}
			return v
		}}).
		Parse(mainGo)
	if nil != err {
		return err
	}
	var text bytes.Buffer
	err = render.Execute(&text, map[string]interface{}{
		"imports": strings.Split(args.packages, ","),
		"name":    args.name,
	})
	if nil != err {
		return err
	}
	source, err := format.Source(text.Bytes())
	if nil != err {
		return err
	}
	args.source = source
	return chain.build(args)
}

func writer(chain *Chain, args *Args) error {
	_, err := os.Stat(args.home)
	if nil != err && os.IsExist(err) {
		return err
	}
	if nil != err {
		if err := os.MkdirAll(args.home, 0777); nil != err {
			return err
		}
	}
	defer func() { _ = os.RemoveAll(args.home) }()

	if err := ioutil.WriteFile(filepath.Join(args.home, "main.go"), args.source, 0777); nil != err {
		return err
	}
	return chain.build(args)
}

func compiler(chain *Chain, args *Args) error {
	build := exec.Command(args.command, "build", fmt.Sprintf("-ldflags=%s", args.flags), "-o", args.target)
	environs := func(matrix ...[]string) []string {
		var environs []string
		for _, envs := range matrix {
			for _, env := range envs {
				if strings.Index(env, "PWD=") == 0 {
					environs = append(environs, fmt.Sprintf("PWD=%s", args.home))
				} else {
					environs = append(environs, env)
				}
			}
		}
		return environs
	}
	build.Env = environs(os.Environ(), strings.Split(args.environ, " "))
	build.Dir = args.home
	output, err := build.CombinedOutput()
	if nil != err {
		return fmt.Errorf("%s\n%s", string(output), err.Error())
	}
	fmt.Println(string(output))
	return chain.build(args)
}
