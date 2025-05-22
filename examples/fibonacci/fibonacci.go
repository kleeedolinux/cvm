package fibonacci

import (
	"clover/vm"
	"fmt"
)

func Run() {
	v := vm.NewVM()
	code := []vm.Instruction{
		{Op: vm.PUSH, Value: 0},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.PUSH, Value: 10},
		{Op: vm.STORE, Value: 0},
		{Op: vm.STORE, Value: 1},
		{Op: vm.STORE, Value: 2},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 2},
		{Op: vm.GT, Value: nil},
		{Op: vm.JMPIF, Value: 27},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.RET, Value: nil},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.LOAD, Value: 1},
		{Op: vm.ADD, Value: nil},
		{Op: vm.LOAD, Value: 1},
		{Op: vm.STORE, Value: 0},
		{Op: vm.STORE, Value: 1},
		{Op: vm.LOAD, Value: 2},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.SUB, Value: nil},
		{Op: vm.STORE, Value: 2},
		{Op: vm.JMP, Value: 6},
	}

	v.Execute(code)
	result := v.Pop()
	fmt.Printf("Fibonacci(10) = %v\n", result)
}
