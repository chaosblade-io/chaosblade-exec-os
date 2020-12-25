package main

import (
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
)

func main() {
	fmt.Println(model.Load(exec.BurnIOBin).Exec().ToString())
}
