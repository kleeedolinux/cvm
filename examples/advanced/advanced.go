package advanced

import (
	"clover/vm"
	"fmt"
	"time"
)

// Run demonstrates the advanced features of the CVM
func Run() {
	// Create a new VM instance
	v := vm.NewVM()

	// Enable debugging
	v.EnableDebug(true)

	// Register a custom type
	v.RegisterType("Person",
		map[string]vm.PropertyInfo{
			"name": {Type: "string", ReadOnly: false},
			"age":  {Type: "int", ReadOnly: false},
		},
		map[string]vm.MethodInfo{
			"greet": {
				Code: []vm.Instruction{
					{Op: vm.PUSH, Value: "Hello, my name is "},
					{Op: vm.STRING, Value: nil},
					{Op: vm.PUSH, Value: 0}, // 'this' object
					{Op: vm.LOAD, Value: 0},
					{Op: vm.PUSH, Value: "name"},
					{Op: vm.FIELD, Value: nil},
					{Op: vm.ADD, Value: nil},
					{Op: vm.PUSH, Value: " and I am "},
					{Op: vm.STRING, Value: nil},
					{Op: vm.ADD, Value: nil},
					{Op: vm.PUSH, Value: 0},
					{Op: vm.LOAD, Value: 0},
					{Op: vm.PUSH, Value: "age"},
					{Op: vm.FIELD, Value: nil},
					{Op: vm.ADD, Value: nil},
					{Op: vm.PUSH, Value: " years old."},
					{Op: vm.STRING, Value: nil},
					{Op: vm.ADD, Value: nil},
					{Op: vm.PRINT, Value: nil},
					{Op: vm.RET, Value: nil},
				},
				ParamCount: 0,
			},
		},
	)

	// Load standard libraries
	v.LoadLib("math")
	v.LoadLib("string")
	v.LoadLib("io")

	// Register a custom function
	v.RegisterFunction("factorial", []vm.Instruction{
		{Op: vm.PUSH, Value: 0},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.LTE, Value: nil},
		{Op: vm.JMPIF, Value: 8},
		{Op: vm.PUSH, Value: 0},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 0},
		{Op: vm.LOAD, Value: 0},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.SUB, Value: nil},
		{Op: vm.CALL, Value: "factorial"},
		{Op: vm.MUL, Value: nil},
		{Op: vm.RET, Value: nil},
		{Op: vm.PUSH, Value: 1},
		{Op: vm.RET, Value: nil},
	})

	// Create main program
	code := []vm.Instruction{
		// Create a Person object
		{Op: vm.NEW_OBJECT, Value: "Person"},

		// Set properties
		{Op: vm.PUSH, Value: "John Doe"},
		{Op: vm.STRING, Value: nil},
		{Op: vm.PUSH, Value: "name"},
		{Op: vm.FIELD, Value: nil},

		{Op: vm.PUSH, Value: 30},
		{Op: vm.PUSH, Value: "age"},
		{Op: vm.FIELD, Value: nil},

		// Store in global variable
		{Op: vm.GLOBAL_SET, Value: "person"},

		// Call a method
		{Op: vm.GLOBAL_GET, Value: "person"},
		{Op: vm.METHOD_CALL, Value: map[string]interface{}{
			"type": "Person",
			"name": "greet",
		}},

		// Calculate factorial with error handling
		{Op: vm.TRY, Value: 23}, // Jump to handler on error
		{Op: vm.PUSH, Value: 5},
		{Op: vm.CALL, Value: "factorial"},
		{Op: vm.PUSH, Value: "Factorial result: "},
		{Op: vm.STRING, Value: nil},
		{Op: vm.SWAP, Value: nil},
		{Op: vm.ADD, Value: nil},
		{Op: vm.PRINT, Value: nil},
		{Op: vm.JMP, Value: 33}, // Skip error handler

		// Error handler
		{Op: vm.PUSH, Value: "Error occurred: "},
		{Op: vm.STRING, Value: nil},
		{Op: vm.SWAP, Value: nil},
		{Op: vm.ADD, Value: nil},
		{Op: vm.PRINT, Value: nil},

		// File I/O
		{Op: vm.PUSH, Value: "test.txt"},
		{Op: vm.STRING, Value: nil},
		{Op: vm.PUSH, Value: "w"},
		{Op: vm.FILE_OPEN, Value: nil},
		{Op: vm.PUSH, Value: "Hello from CVM!"},
		{Op: vm.STRING, Value: nil},
		{Op: vm.FILE_WRITE, Value: nil},
		{Op: vm.FILE_CLOSE, Value: nil},

		// Concurrent execution
		{Op: vm.CHANNEL, Value: 1},
		{Op: vm.STORE, Value: 0},

		// Producer
		{Op: vm.PUSH, Value: []vm.Instruction{
			{Op: vm.PUSH, Value: "Starting producer..."},
			{Op: vm.STRING, Value: nil},
			{Op: vm.PRINT, Value: nil},
			{Op: vm.PUSH, Value: 0},
			{Op: vm.LOAD, Value: 0},
			{Op: vm.PUSH, Value: "Hello from producer!"},
			{Op: vm.STRING, Value: nil},
			{Op: vm.SEND, Value: nil},
			{Op: vm.PUSH, Value: 500},
			{Op: vm.TIMER, Value: time.Millisecond * 500},
			{Op: vm.POP, Value: nil},
		}},
		{Op: vm.SPAWN, Value: nil},

		// Consumer
		{Op: vm.PUSH, Value: []vm.Instruction{
			{Op: vm.PUSH, Value: "Starting consumer..."},
			{Op: vm.STRING, Value: nil},
			{Op: vm.PRINT, Value: nil},
			{Op: vm.PUSH, Value: 0},
			{Op: vm.LOAD, Value: 0},
			{Op: vm.RECV, Value: nil},
			{Op: vm.PRINT, Value: nil},
		}},
		{Op: vm.SPAWN, Value: nil},

		// Wait a bit
		{Op: vm.PUSH, Value: 1000},
		{Op: vm.TIMER, Value: time.Millisecond * 1000},
		{Op: vm.POP, Value: nil},

		{Op: vm.PUSH, Value: "Advanced example completed!"},
		{Op: vm.STRING, Value: nil},
		{Op: vm.PRINT, Value: nil},
	}

	// Execute the program
	fmt.Println("Running advanced example...")
	v.Execute(code)
	fmt.Println("Done!")
}
