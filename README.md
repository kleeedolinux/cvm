# Clover Virtual Machine (CVM)

Clover is a high-performance, concurrent virtual machine written in Go. It features a rich instruction set, efficient garbage collection, and support for complex data structures.

## Features

### Core Features
- Stack-based virtual machine architecture
- Concurrent execution with goroutines
- Channel-based communication
- Efficient garbage collection
- Complex data structure support
- Object persistence
- Instruction optimization

### Data Structures
- Lists (dynamic arrays)
- Dictionaries (key-value maps)
- Structs (field-based objects)
- Strings (with concatenation support)
- Channels (for concurrent communication)

### Memory Management
- Reference counting
- Background garbage collection
- Automatic cleanup of unused objects
- Thread-safe operations
- Object persistence to disk

## Instruction Set

### Stack Operations
- `PUSH` - Push value onto stack
- `POP` - Pop value from stack
- `DUP` - Duplicate top stack value
- `SWAP` - Swap top two stack values

### Arithmetic Operations
- `ADD` - Add two values
- `SUB` - Subtract two values
- `MUL` - Multiply two values
- `DIV` - Divide two values
- `MOD` - Modulo operation

### Comparison Operations
- `EQ` - Equal to
- `NEQ` - Not equal to
- `GT` - Greater than
- `LT` - Less than
- `GTE` - Greater than or equal
- `LTE` - Less than or equal

### Control Flow
- `JMP` - Unconditional jump
- `JMPIF` - Jump if true
- `JMPIFNOT` - Jump if false
- `CALL` - Call function
- `RET` - Return from function

### Memory Operations
- `LOAD` - Load value from memory
- `STORE` - Store value in memory

### Concurrency Operations
- `SPAWN` - Spawn new routine
- `YIELD` - Yield execution
- `JOIN` - Join routine
- `CHANNEL` - Create channel
- `SEND` - Send to channel
- `RECV` - Receive from channel
- `SELECT` - Select from multiple channels
- `TIMER` - Create timer

### Complex Data Structures
- `LIST` - Create list
- `DICT` - Create dictionary
- `STRUCT` - Create struct
- `STRING` - Create string
- `APPEND` - Append to list
- `SET` - Set dictionary value
- `FIELD` - Set struct field
- `PERSIST` - Persist object
- `LOAD_PERSISTED` - Load persisted object

## Examples

### Fibonacci Example
```go
func Run() {
    v := vm.NewVM()
    code := []vm.Instruction{
        {Op: vm.PUSH, Value: 10},
        {Op: vm.CALL, Value: "fibonacci"},
        {Op: vm.RET, Value: nil},
    }
    v.Execute(code)
}
```

### Concurrency Example
```go
func Run() {
    v := vm.NewVM()
    producer := []vm.Instruction{
        {Op: vm.PUSH, Value: 1},
        {Op: vm.CHANNEL, Value: 1},
        {Op: vm.STORE, Value: 0},
        // ... producer logic
    }
    consumer := []vm.Instruction{
        {Op: vm.PUSH, Value: 0},
        {Op: vm.STORE, Value: 0},
        // ... consumer logic
    }
    code := []vm.Instruction{
        {Op: vm.PUSH, Value: producer},
        {Op: vm.SPAWN, Value: producer},
        {Op: vm.PUSH, Value: consumer},
        {Op: vm.SPAWN, Value: consumer},
    }
    v.Execute(code)
}
```

### Complex Data Structure Example
```go
func Run() {
    v := vm.NewVM()
    code := []vm.Instruction{
        // Create a list
        {Op: vm.LIST, Value: nil},
        // Create a dictionary
        {Op: vm.DICT, Value: nil},
        // Create a struct
        {Op: vm.STRUCT, Value: map[string]interface{}{
            "name": "value",
        }},
        // Append to list
        {Op: vm.PUSH, Value: "item"},
        {Op: vm.APPEND, Value: nil},
        // Set dictionary value
        {Op: vm.PUSH, Value: "key"},
        {Op: vm.PUSH, Value: "value"},
        {Op: vm.SET, Value: nil},
        // Set struct field
        {Op: vm.PUSH, Value: "field"},
        {Op: vm.PUSH, Value: "value"},
        {Op: vm.FIELD, Value: nil},
    }
    v.Execute(code)
}
```

## Garbage Collection

The CVM includes an efficient garbage collector with the following features:

### Reference Counting
- Automatic tracking of object references
- Immediate cleanup of unreferenced objects
- Thread-safe reference counting

### Background Collection
- Periodic cleanup of unused objects
- Configurable collection intervals
- Non-blocking collection process

### Persistence
- JSON-based object serialization
- File-based storage
- Automatic loading of persisted objects
- Metadata support for objects

## Performance Optimizations

### Memory Management
- Efficient object allocation
- Minimal memory fragmentation
- Smart object reuse

### Concurrency
- Lock-free operations where possible
- Minimal locking scope
- Efficient channel operations

### Instruction Optimization
- Static instruction analysis
- Dead code elimination
- Constant folding
- Instruction reordering

## Getting Started

1. Clone the repository:
```bash
git clone https://github.com/kleeedolinux/cvm.git
cd cvm
```

2. Run the examples:
```bash
go run examples/main.go
```

3. Create your own programs:
```go
package main

import "clover/vm"

func main() {
    v := vm.NewVM()
    // Your code here
    v.Execute(code)
}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 