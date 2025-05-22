package concurrency

import (
	"clover/vm"
	"fmt"
	"time"
)

func Run() {
	v := vm.NewVM()

	producer := []vm.Instruction{
		{Op: vm.PUSH, Value: 1},
		{Op: vm.CHANNEL, Value: 1},
		{Op: vm.STORE, Value: 0},
		{Op: vm.PUSH, Value: 0},
		{Op: vm.STORE, Value: 1},
		{Op: vm.LOAD, Value: 1},
		{Op: vm.PUSH, Value: 5},
		{Op: vm.LT, Value: nil},
		{Op: vm.JMPIF, Value: 12},
		{Op: vm.RET, Value: nil},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.SEND, Value: nil},
		{Op: vm.LOAD, Value: 1},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.ADD, Value: nil},
		{Op: vm.STORE, Value: 1},
		{Op: vm.PUSH, Value: 100},
		{Op: vm.TIMER, Value: time.Millisecond * 100},
		{Op: vm.YIELD, Value: nil},
		{Op: vm.JMP, Value: 5},
	}

	consumer := []vm.Instruction{
		{Op: vm.PUSH, Value: 0},
		{Op: vm.STORE, Value: 0},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 5},
		{Op: vm.LT, Value: nil},
		{Op: vm.JMPIF, Value: 8},
		{Op: vm.RET, Value: nil},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.RECV, Value: nil},
		{Op: vm.POP, Value: nil},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.ADD, Value: nil},
		{Op: vm.STORE, Value: 0},
		{Op: vm.JMP, Value: 2},
	}

	code := []vm.Instruction{
		{Op: vm.PUSH, Value: producer},
		{Op: vm.SPAWN, Value: producer},
		{Op: vm.PUSH, Value: consumer},
		{Op: vm.SPAWN, Value: consumer},
	}

	fmt.Println("Starting concurrency example...")
	v.Execute(code)
	time.Sleep(time.Second * 2)
	fmt.Println("Concurrency example completed")
}
