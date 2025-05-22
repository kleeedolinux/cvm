package vm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

type Value interface{}

type VM struct {
	stack        []Value
	memory       map[int]Value
	pc           int
	code         []Instruction
	mutex        sync.Mutex
	routines     map[int]*VM
	routineID    int
	optimizer    *Optimizer
	channels     map[int]*Channel
	channelID    int
	gc           *GC
	functions    map[string][]Instruction
	callStack    []int
	errorHandler func(error)
	stdOut       io.Writer
	stdIn        io.Reader
	debug        bool
	globals      map[string]Value
	types        map[string]*TypeInfo
}

type TypeInfo struct {
	Name       string
	Properties map[string]PropertyInfo
	Methods    map[string]MethodInfo
}

type PropertyInfo struct {
	Type     string
	ReadOnly bool
}

type MethodInfo struct {
	Code       []Instruction
	ParamCount int
}

func NewVM() *VM {
	return &VM{
		stack:        make([]Value, 0, 1024),
		memory:       make(map[int]Value),
		routines:     make(map[int]*VM),
		channels:     make(map[int]*Channel),
		optimizer:    NewOptimizer(1000),
		gc:           NewGC("./persist"),
		functions:    make(map[string][]Instruction),
		callStack:    make([]int, 0, 64),
		errorHandler: defaultErrorHandler,
		stdOut:       os.Stdout,
		stdIn:        os.Stdin,
		globals:      make(map[string]Value),
		types:        make(map[string]*TypeInfo),
	}
}

func defaultErrorHandler(err error) {
	fmt.Fprintf(os.Stderr, "CVM Error: %v\n", err)
}

func (vm *VM) SetErrorHandler(handler func(error)) {
	vm.errorHandler = handler
}

func (vm *VM) SetStdOut(w io.Writer) {
	vm.stdOut = w
}

func (vm *VM) SetStdIn(r io.Reader) {
	vm.stdIn = r
}

func (vm *VM) EnableDebug(enable bool) {
	vm.debug = enable
}

func (vm *VM) RegisterFunction(name string, code []Instruction) {
	vm.functions[name] = code
}

func (vm *VM) RegisterType(name string, properties map[string]PropertyInfo, methods map[string]MethodInfo) {
	vm.types[name] = &TypeInfo{
		Name:       name,
		Properties: properties,
		Methods:    methods,
	}
}

func (vm *VM) GetGlobal(name string) Value {
	return vm.globals[name]
}

func (vm *VM) SetGlobal(name string, value Value) {
	vm.globals[name] = value
}

func (vm *VM) Push(v Value) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	if id, ok := v.(uint64); ok {
		vm.gc.IncrementRef(id)
	}
	vm.stack = append(vm.stack, v)
}

func (vm *VM) Pop() Value {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	if len(vm.stack) == 0 {
		return nil
	}
	v := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	if id, ok := v.(uint64); ok {
		vm.gc.DecrementRef(id)
	}
	return v
}

func (vm *VM) Load(addr int) Value {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	v := vm.memory[addr]
	if id, ok := v.(uint64); ok {
		vm.gc.IncrementRef(id)
	}
	return v
}

func (vm *VM) Store(addr int, v Value) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	if oldID, ok := vm.memory[addr].(uint64); ok {
		vm.gc.DecrementRef(oldID)
	}
	if newID, ok := v.(uint64); ok {
		vm.gc.IncrementRef(newID)
	}
	vm.memory[addr] = v
}

func (vm *VM) Execute(code []Instruction) error {
	vm.code = vm.optimizer.Optimize(code)
	vm.pc = 0

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				vm.handleError(err)
			} else {
				vm.handleError(fmt.Errorf("VM panic: %v", r))
			}
		}
	}()

	for vm.pc < len(vm.code) {
		if vm.debug {
			fmt.Fprintf(vm.stdOut, "PC: %d, Instruction: %v\n", vm.pc, vm.code[vm.pc])
		}
		instr := vm.code[vm.pc]
		if err := vm.executeInstruction(instr); err != nil {
			return err
		}
		vm.pc++
	}
	return nil
}

func (vm *VM) handleError(err error) {
	if vm.errorHandler != nil {
		vm.errorHandler(err)
	}
}

func (vm *VM) executeInstruction(instr Instruction) error {
	switch instr.Op {
	case PUSH:
		vm.Push(instr.Value)
	case POP:
		vm.Pop()
	case ADD:
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				vm.Push(aInt + bInt)
			}
		} else if aID, ok := a.(uint64); ok {
			if bID, ok := b.(uint64); ok {
				aObj := vm.gc.Get(aID)
				bObj := vm.gc.Get(bID)
				if aObj != nil && bObj != nil {
					if aObj.Type == TypeString && bObj.Type == TypeString {
						newID := vm.gc.CreateString(aObj.Value.(string) + bObj.Value.(string))
						vm.Push(newID)
					}
				}
			}
		}
	case SUB:
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				vm.Push(aInt - bInt)
			}
		}
	case MUL:
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				vm.Push(aInt * bInt)
			}
		}
	case DIV:
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				if bInt == 0 {
					return errors.New("division by zero")
				}
				vm.Push(aInt / bInt)
			}
		}
	case MOD:
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				if bInt == 0 {
					return errors.New("modulo by zero")
				}
				vm.Push(aInt % bInt)
			}
		}
	case LOAD:
		if addr, ok := instr.Value.(int); ok {
			vm.Push(vm.Load(addr))
		}
	case STORE:
		if addr, ok := instr.Value.(int); ok {
			v := vm.Pop()
			vm.Store(addr, v)
		}
	case JMP:
		if addr, ok := instr.Value.(int); ok {
			vm.pc = addr - 1
		}
	case JMPIF:
		if addr, ok := instr.Value.(int); ok {
			if cond := vm.Pop(); cond != nil && cond != false {
				vm.pc = addr - 1
			}
		}
	case JMPIFNOT:
		if addr, ok := instr.Value.(int); ok {
			if cond := vm.Pop(); cond == nil || cond == false {
				vm.pc = addr - 1
			}
		}
	case CALL:
		if name, ok := instr.Value.(string); ok {
			if code, exists := vm.functions[name]; exists {
				vm.callStack = append(vm.callStack, vm.pc)
				savedCode := vm.code
				vm.code = code
				vm.pc = -1 // Will be incremented to 0 after return
				defer func() {
					vm.pc = vm.callStack[len(vm.callStack)-1]
					vm.callStack = vm.callStack[:len(vm.callStack)-1]
					vm.code = savedCode
				}()
			} else {
				return fmt.Errorf("function not found: %s", name)
			}
		}
	case RET:
		if len(vm.callStack) > 0 {
			vm.pc = vm.callStack[len(vm.callStack)-1]
			vm.callStack = vm.callStack[:len(vm.callStack)-1]
		}
	case SPAWN:
		if code, ok := instr.Value.([]Instruction); ok {
			newVM := NewVM()
			newVM.routineID = vm.routineID + 1
			vm.routines[newVM.routineID] = newVM
			go newVM.Execute(code)
			vm.Push(newVM.routineID)
		}
	case YIELD:
		runtime.Gosched()
	case JOIN:
		if routineID, ok := vm.Pop().(int); ok {
			if _, exists := vm.routines[routineID]; exists {
				delete(vm.routines, routineID)
			}
		}
	case CHANNEL:
		if capacity, ok := instr.Value.(int); ok {
			ch := NewChannel(capacity)
			vm.channelID++
			vm.channels[vm.channelID] = ch
			vm.Push(vm.channelID)
		}
	case SEND:
		if chID, ok := vm.Pop().(int); ok {
			if ch, exists := vm.channels[chID]; exists {
				value := vm.Pop()
				ch.Send(value)
			}
		}
	case RECV:
		if chID, ok := vm.Pop().(int); ok {
			if ch, exists := vm.channels[chID]; exists {
				if value, ok := ch.Receive(); ok {
					vm.Push(value)
				}
			}
		}
	case SELECT:
		if cases, ok := instr.Value.([]SelectCase); ok {
			if index, ok := Select(cases); ok {
				vm.Push(index)
			}
		}
	case TIMER:
		if duration, ok := instr.Value.(time.Duration); ok {
			timer := NewTimer(duration)
			vm.Push(timer)
		}
	case LIST:
		vm.Push(vm.gc.CreateList())
	case DICT:
		vm.Push(vm.gc.CreateDict())
	case STRUCT:
		if fields, ok := instr.Value.(map[string]interface{}); ok {
			vm.Push(vm.gc.CreateStruct(fields))
		}
	case STRING:
		if str, ok := instr.Value.(string); ok {
			vm.Push(vm.gc.CreateString(str))
		}
	case APPEND:
		value := vm.Pop()
		listID := vm.Pop().(uint64)
		vm.gc.ListAppend(listID, value)
	case SET:
		value := vm.Pop()
		key := vm.Pop().(string)
		dictID := vm.Pop().(uint64)
		vm.gc.DictSet(dictID, key, value)
	case FIELD:
		value := vm.Pop()
		field := vm.Pop().(string)
		structID := vm.Pop().(uint64)
		vm.gc.StructSet(structID, field, value)
	case PERSIST:
		if id, ok := vm.Pop().(uint64); ok {
			vm.gc.Persist(id)
		}
	case LOAD_PERSISTED:
		if id, ok := instr.Value.(uint64); ok {
			vm.gc.Load(id)
		}
	case PRINT:
		value := vm.Pop()
		fmt.Fprintln(vm.stdOut, value)
	case READ:
		var input string
		fmt.Fscanln(vm.stdIn, &input)
		strID := vm.gc.CreateString(input)
		vm.Push(strID)
	case GLOBAL_GET:
		if name, ok := instr.Value.(string); ok {
			vm.Push(vm.globals[name])
		}
	case GLOBAL_SET:
		if name, ok := instr.Value.(string); ok {
			value := vm.Pop()
			vm.globals[name] = value
		}
	case NEW_OBJECT:
		if typeName, ok := instr.Value.(string); ok {
			if typeInfo, exists := vm.types[typeName]; exists {
				objID := vm.gc.CreateStruct(make(map[string]interface{}))
				for name, prop := range typeInfo.Properties {
					var defaultValue interface{}
					switch prop.Type {
					case "int":
						defaultValue = 0
					case "string":
						defaultValue = vm.gc.CreateString("")
					case "list":
						defaultValue = vm.gc.CreateList()
					case "dict":
						defaultValue = vm.gc.CreateDict()
					}
					vm.gc.StructSet(objID, name, defaultValue)
				}
				vm.Push(objID)
			}
		}
	case METHOD_CALL:
		if methodData, ok := instr.Value.(map[string]interface{}); ok {
			objID := vm.Pop().(uint64)
			methodName := methodData["name"].(string)
			typeInfo := vm.types[methodData["type"].(string)]
			if method, exists := typeInfo.Methods[methodName]; exists {
				// Save current state
				savedPC := vm.pc
				savedCode := vm.code

				// Execute method
				vm.callStack = append(vm.callStack, savedPC)
				vm.code = method.Code
				vm.pc = -1 // Will be incremented to 0 in main loop

				// Push object as 'this'
				vm.Push(objID)

				defer func() {
					// Restore state after method execution
					vm.pc = vm.callStack[len(vm.callStack)-1]
					vm.callStack = vm.callStack[:len(vm.callStack)-1]
					vm.code = savedCode
				}()
			}
		}
	case TRY:
		if handlerAddr, ok := instr.Value.(int); ok {
			defer func() {
				if r := recover(); r != nil {
					// Jump to handler on error
					vm.pc = handlerAddr - 1
					if err, ok := r.(error); ok {
						errStrID := vm.gc.CreateString(err.Error())
						vm.Push(errStrID)
					} else {
						errStrID := vm.gc.CreateString(fmt.Sprintf("%v", r))
						vm.Push(errStrID)
					}
				}
			}()
		}
	case THROW:
		errMsg := vm.Pop()
		if strID, ok := errMsg.(uint64); ok {
			obj := vm.gc.Get(strID)
			if obj != nil && obj.Type == TypeString {
				panic(errors.New(obj.Value.(string)))
			}
		} else {
			panic(fmt.Sprintf("%v", errMsg))
		}
	case FILE_OPEN:
		if modeVal, ok := vm.Pop().(string); ok {
			if pathVal, ok := vm.Pop().(uint64); ok {
				obj := vm.gc.Get(pathVal)
				if obj != nil && obj.Type == TypeString {
					path := obj.Value.(string)
					var file *os.File
					var err error

					switch modeVal {
					case "r":
						file, err = os.Open(path)
					case "w":
						file, err = os.Create(path)
					case "a":
						file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					default:
						return fmt.Errorf("invalid file mode: %s", modeVal)
					}

					if err != nil {
						return err
					}

					// Store file in globals with unique ID
					fileID := fmt.Sprintf("__file_%p", file)
					vm.globals[fileID] = file

					// Return file ID to stack
					idStrID := vm.gc.CreateString(fileID)
					vm.Push(idStrID)
				}
			}
		}
	case FILE_CLOSE:
		if fileIDVal, ok := vm.Pop().(uint64); ok {
			obj := vm.gc.Get(fileIDVal)
			if obj != nil && obj.Type == TypeString {
				fileID := obj.Value.(string)
				if file, ok := vm.globals[fileID].(*os.File); ok {
					file.Close()
					delete(vm.globals, fileID)
				}
			}
		}
	case FILE_READ:
		if fileIDVal, ok := vm.Pop().(uint64); ok {
			obj := vm.gc.Get(fileIDVal)
			if obj != nil && obj.Type == TypeString {
				fileID := obj.Value.(string)
				if file, ok := vm.globals[fileID].(*os.File); ok {
					data := make([]byte, 1024)
					n, err := file.Read(data)
					if err != nil && err != io.EOF {
						return err
					}

					content := string(data[:n])
					contentID := vm.gc.CreateString(content)
					vm.Push(contentID)
				}
			}
		}
	case FILE_WRITE:
		if contentVal, ok := vm.Pop().(uint64); ok {
			if fileIDVal, ok := vm.Pop().(uint64); ok {
				contentObj := vm.gc.Get(contentVal)
				fileIDObj := vm.gc.Get(fileIDVal)

				if contentObj != nil && contentObj.Type == TypeString &&
					fileIDObj != nil && fileIDObj.Type == TypeString {

					content := contentObj.Value.(string)
					fileID := fileIDObj.Value.(string)

					if file, ok := vm.globals[fileID].(*os.File); ok {
						_, err := file.WriteString(content)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (vm *VM) LoadLib(name string) error {
	switch name {
	case "math":
		vm.RegisterFunction("abs", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: PUSH, Value: 0},
			{Op: LT, Value: nil},
			{Op: JMPIF, Value: 8},
			{Op: LOAD, Value: 0},
			{Op: RET, Value: nil},
			{Op: LOAD, Value: 0},
			{Op: PUSH, Value: -1},
			{Op: MUL, Value: nil},
			{Op: RET, Value: nil},
		})
		vm.RegisterFunction("max", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: PUSH, Value: 1},
			{Op: LOAD, Value: 1},
			{Op: GT, Value: nil},
			{Op: JMPIF, Value: 11},
			{Op: PUSH, Value: 1},
			{Op: LOAD, Value: 1},
			{Op: RET, Value: nil},
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: RET, Value: nil},
		})
		vm.RegisterFunction("min", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: PUSH, Value: 1},
			{Op: LOAD, Value: 1},
			{Op: LT, Value: nil},
			{Op: JMPIF, Value: 11},
			{Op: PUSH, Value: 1},
			{Op: LOAD, Value: 1},
			{Op: RET, Value: nil},
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: RET, Value: nil},
		})
	case "string":
		vm.RegisterFunction("len", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			// This would be implemented with native operation
			{Op: RET, Value: nil},
		})
		vm.RegisterFunction("substr", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: PUSH, Value: 1},
			{Op: LOAD, Value: 1},
			{Op: PUSH, Value: 2},
			{Op: LOAD, Value: 2},
			// This would be implemented with native operation
			{Op: RET, Value: nil},
		})
	case "io":
		vm.RegisterFunction("println", []Instruction{
			{Op: PUSH, Value: 0},
			{Op: LOAD, Value: 0},
			{Op: PRINT, Value: nil},
			{Op: RET, Value: nil},
		})
		vm.RegisterFunction("readln", []Instruction{
			{Op: READ, Value: nil},
			{Op: RET, Value: nil},
		})
	default:
		return fmt.Errorf("unknown library: %s", name)
	}
	return nil
}
