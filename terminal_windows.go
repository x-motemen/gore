package main

import (
	"syscall"
	"unsafe"
)

type short int16
type word uint16

type coord struct {
	x short
	y short
}

type smallRect struct {
	left   short
	top    short
	right  short
	bottom short
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        word
	window            smallRect
	maximumWindowSize coord
}

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle               = kernel32.NewProc("GetStdHandle")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")

	stdoutHandle uintptr
)

func init() {
	stdoutHandle = getStdHandle(syscall.STD_OUTPUT_HANDLE)
}

func getStdHandle(stdhandle int32) uintptr {
	handle, _, _ := procGetStdHandle.Call(uintptr(stdhandle))
	return handle
}

func cursorUp() {
	var csbi consoleScreenBufferInfo
	procGetConsoleScreenBufferInfo.Call(stdoutHandle, uintptr(unsafe.Pointer(&csbi)))

	var cursor coord
	cursor.x = csbi.cursorPosition.x
	cursor.y = csbi.cursorPosition.y - 1

	procSetConsoleCursorPosition.Call(stdoutHandle, uintptr(*(*int32)(unsafe.Pointer(&cursor))))
}

func eraseInLine() {
	var csbi consoleScreenBufferInfo
	procGetConsoleScreenBufferInfo.Call(stdoutHandle, uintptr(unsafe.Pointer(&csbi)))

	var w uint32
	procFillConsoleOutputCharacter.Call(stdoutHandle, uintptr(' '), uintptr(csbi.size.x), uintptr(*(*int32)(unsafe.Pointer(&csbi.cursorPosition))), uintptr(unsafe.Pointer(&w)))
}
