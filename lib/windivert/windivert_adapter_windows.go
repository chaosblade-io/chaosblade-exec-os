package windivert

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

var windivertDLL *syscall.DLL
var open, recv, send, close, setParam, getParam, calcChecksums, parsePacket *syscall.Proc

// load windivert DLL
func init() {
	// 兼容blade和os两块使用这块代码
	var windivertHome string
	programPath := util.GetProgramPath()
	dir, file := util.Split(programPath)
	if file == "bin" {
		windivertHome = fmt.Sprintf("%s\\lib\\windivert", dir)
	} else {
		windivertHome = fmt.Sprintf("%s\\lib\\windivert", programPath)
	}

	windivertDLL = syscall.MustLoadDLL(fmt.Sprintf("%s\\WinDivert.dll", windivertHome))
	open = windivertDLL.MustFindProc("WinDivertOpen")
	recv = windivertDLL.MustFindProc("WinDivertRecv")
	send = windivertDLL.MustFindProc("WinDivertSend")
	close = windivertDLL.MustFindProc("WinDivertClose")
	setParam = windivertDLL.MustFindProc("WinDivertSetParam")
	getParam = windivertDLL.MustFindProc("WinDivertGetParam")
	calcChecksums = windivertDLL.MustFindProc("WinDivertHelperCalcChecksums")
	parsePacket = windivertDLL.MustFindProc("WinDivertHelperParsePacket")
}

func NewWindivertAdapter(filter string) (*WindivertAdapter, error) {

	filterBytePtr, err := syscall.BytePtrFromString(filter)
	if err != nil {
		return nil, err
	}

	handle, _, err := open.Call(uintptr(unsafe.Pointer(filterBytePtr)),
		uintptr(0),
		uintptr(0),
		uintptr(0))

	if handle == uintptr(syscall.InvalidHandle) {
		return nil, err
	}

	return &WindivertAdapter{
		filter: filter,
		handle: handle,
	}, nil
}

const BufferSize = 1024

type WindivertAdapter struct {
	filter string
	handle uintptr
}

func (w *WindivertAdapter) Recv() ([]byte, WinDivertAddress) {

	buffer := make([]byte, BufferSize)
	var length uint
	var addr WinDivertAddress

	recv.Call(w.handle,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(BufferSize),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&length)))

	i := buffer[:length]
	return i, addr
}

func (w *WindivertAdapter) Send(raw []byte, addr WinDivertAddress) error {

	var length uint

	send.Call(w.handle,
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(len(raw)),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&length)))

	return nil
}
