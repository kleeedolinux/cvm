package vm

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
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
	memManager   *MemoryManager
	crypto       *CryptoModule
	modules      map[string]*Module
	imports      map[string]bool
	context      *Context
	exports      map[string]bool
	deferred     [][]Instruction
}

type Context struct {
	Variables map[string]Value
	Parent    *Context
	Scope     int
}

type Module struct {
	Name      string
	Functions map[string][]Instruction
	Globals   map[string]Value
	Types     map[string]*TypeInfo
	Imports   map[string]bool
	Exports   map[string]bool
}

type TypeInfo struct {
	Name       string
	Properties map[string]PropertyInfo
	Methods    map[string]MethodInfo
	BaseType   string
	Interfaces []string
	Generics   []string
}

type PropertyInfo struct {
	Type     string
	ReadOnly bool
	Default  interface{}
	Tags     map[string]string
}

type MethodInfo struct {
	Code       []Instruction
	ParamCount int
	Returns    []string
	Variadic   bool
	Async      bool
	Decorators []string
}

func NewVM() *VM {
	vm := &VM{
		stack:        make([]Value, 0, 4096),
		memory:       make(map[int]Value, 1024),
		routines:     make(map[int]*VM, 32),
		channels:     make(map[int]*Channel, 32),
		optimizer:    NewOptimizer(1000),
		gc:           NewGC("./persist"),
		functions:    make(map[string][]Instruction, 64),
		callStack:    make([]int, 0, 128),
		errorHandler: defaultErrorHandler,
		stdOut:       os.Stdout,
		stdIn:        os.Stdin,
		globals:      make(map[string]Value, 64),
		types:        make(map[string]*TypeInfo, 32),
		memManager:   NewMemoryManager(),
		modules:      make(map[string]*Module),
		imports:      make(map[string]bool),
		context:      &Context{Variables: make(map[string]Value)},
		exports:      make(map[string]bool),
		deferred:     make([][]Instruction, 0),
	}
	vm.crypto = NewCryptoModule(vm)
	vm.crypto.RegisterFunctions()
	return vm
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

func (vm *VM) Push(values ...interface{}) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	for _, v := range values {
		if id, ok := v.(uint64); ok {
			vm.gc.IncrementRef(id)
		}
		vm.stack = append(vm.stack, v)
	}
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

func (vm *VM) allocateMemory(size uintptr) (uintptr, error) {
	return vm.memManager.allocate(size)
}

func (vm *VM) freeMemory(ptr uintptr) {
	vm.memManager.free(ptr)
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

type instructionHandler func(*VM, Instruction) error

var instructionHandlers = map[OpCode]instructionHandler{
	PUSH: func(vm *VM, instr Instruction) error {
		vm.Push(instr.Value)
		return nil
	},
	POP: func(vm *VM, instr Instruction) error {
		vm.Pop()
		return nil
	},
	ADD: func(vm *VM, instr Instruction) error {
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
		return nil
	},
	SUB: func(vm *VM, instr Instruction) error {
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				vm.Push(aInt - bInt)
			}
		}
		return nil
	},
	MUL: func(vm *VM, instr Instruction) error {
		b := vm.Pop()
		a := vm.Pop()
		if aInt, ok := a.(int); ok {
			if bInt, ok := b.(int); ok {
				vm.Push(aInt * bInt)
			}
		}
		return nil
	},
	DIV: func(vm *VM, instr Instruction) error {
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
		return nil
	},
	MOD: func(vm *VM, instr Instruction) error {
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
		return nil
	},
	LOAD: func(vm *VM, instr Instruction) error {
		if addr, ok := instr.Value.(int); ok {
			vm.Push(vm.Load(addr))
		}
		return nil
	},
	STORE: func(vm *VM, instr Instruction) error {
		if addr, ok := instr.Value.(int); ok {
			v := vm.Pop()
			vm.Store(addr, v)
		}
		return nil
	},
	JMP: func(vm *VM, instr Instruction) error {
		if addr, ok := instr.Value.(int); ok {
			vm.pc = addr - 1
		}
		return nil
	},
	JMPIF: func(vm *VM, instr Instruction) error {
		if addr, ok := instr.Value.(int); ok {
			if cond := vm.Pop(); cond != nil && cond != false {
				vm.pc = addr - 1
			}
		}
		return nil
	},
	JMPIFNOT: func(vm *VM, instr Instruction) error {
		if addr, ok := instr.Value.(int); ok {
			if cond := vm.Pop(); cond == nil || cond == false {
				vm.pc = addr - 1
			}
		}
		return nil
	},
}

func (vm *VM) executeInstruction(instr Instruction) error {
	if handler, ok := instructionHandlers[instr.Op]; ok {
		return handler(vm, instr)
	}

	switch instr.Op {
	case CALL:
		if name, ok := instr.Value.(string); ok {
			switch name {
			case "__crypto_sha256":
				data := vm.Pop().([]byte)
				hash := vm.crypto.SHA256(data)
				vm.Push(hash)
			case "__crypto_aes_encrypt":
				key := vm.Pop().([]byte)
				plaintext := vm.Pop().([]byte)
				ciphertext, err := vm.crypto.AESEncrypt(key, plaintext)
				if err != nil {
					return err
				}
				vm.Push(ciphertext)
			case "__crypto_aes_decrypt":
				key := vm.Pop().([]byte)
				ciphertext := vm.Pop().([]byte)
				plaintext, err := vm.crypto.AESDecrypt(key, ciphertext)
				if err != nil {
					return err
				}
				vm.Push(plaintext)
			case "__crypto_rsa_generate":
				bits := vm.Pop().(int)
				key, err := vm.crypto.RSAGenerate(bits)
				if err != nil {
					return err
				}
				vm.Push(key)
			case "__crypto_rsa_encrypt":
				plaintext := vm.Pop().([]byte)
				publicKey := vm.Pop().(*rsa.PublicKey)
				ciphertext, err := vm.crypto.RSAEncrypt(publicKey, plaintext)
				if err != nil {
					return err
				}
				vm.Push(ciphertext)
			case "__crypto_rsa_decrypt":
				ciphertext := vm.Pop().([]byte)
				privateKey := vm.Pop().(*rsa.PrivateKey)
				plaintext, err := vm.crypto.RSADecrypt(privateKey, ciphertext)
				if err != nil {
					return err
				}
				vm.Push(plaintext)
			default:
				if code, exists := vm.functions[name]; exists {
					vm.callStack = append(vm.callStack, vm.pc)
					savedCode := vm.code
					vm.code = code
					vm.pc = -1
					defer func() {
						vm.pc = vm.callStack[len(vm.callStack)-1]
						vm.callStack = vm.callStack[:len(vm.callStack)-1]
						vm.code = savedCode
					}()
				} else {
					return fmt.Errorf("function not found: %s", name)
				}
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
				savedPC := vm.pc
				savedCode := vm.code
				vm.callStack = append(vm.callStack, savedPC)
				vm.code = method.Code
				vm.pc = -1
				vm.Push(objID)
				defer func() {
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

					fileID := fmt.Sprintf("__file_%p", file)
					vm.globals[fileID] = file
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
	case IMPORT:
		if moduleName, ok := instr.Value.(string); ok {
			if err := vm.importModule(moduleName); err != nil {
				return err
			}
		}
	case EXPORT:
		if name, ok := instr.Value.(string); ok {
			vm.exports[name] = true
		}
	case DEFER:
		if code, ok := instr.Value.([]Instruction); ok {
			vm.callStack = append(vm.callStack, -1)
			vm.deferred = append(vm.deferred, code)
		}
	case RECOVER:
		if err := vm.Pop(); err != nil {
			vm.Push(err)
		}
	case RANGE:
		if iter, ok := vm.Pop().(Iterator); ok {
			if iter.HasNext() {
				vm.Push(iter.Next())
			} else {
				vm.pc = instr.Value.(int) - 1
			}
		}
	case YIELD_FROM:
		if gen, ok := vm.Pop().(*Generator); ok {
			if value, done := gen.Next(); !done {
				vm.Push(value)
			} else {
				vm.pc = instr.Value.(int) - 1
			}
		}
	case AWAIT:
		if promise, ok := vm.Pop().(*Promise); ok {
			if value, err := promise.Await(); err == nil {
				vm.Push(value)
			} else {
				return err
			}
		}
	case CLASS:
		if classInfo, ok := instr.Value.(map[string]interface{}); ok {
			vm.defineClass(classInfo)
		}
	case INTERFACE:
		if interfaceInfo, ok := instr.Value.(map[string]interface{}); ok {
			vm.defineInterface(interfaceInfo)
		}
	case GENERIC:
		if genericInfo, ok := instr.Value.(map[string]interface{}); ok {
			vm.defineGeneric(genericInfo)
		}
	case DECORATOR:
		if decoratorInfo, ok := instr.Value.(map[string]interface{}); ok {
			vm.applyDecorator(decoratorInfo)
		}
	case PATTERN_MATCH:
		if pattern, ok := instr.Value.(Pattern); ok {
			if value := vm.Pop(); pattern.Match(value) {
				vm.Push(pattern.Extract(value))
			} else {
				vm.pc = instr.Value.(int) - 1
			}
		}
	case SPREAD:
		if collection, ok := vm.Pop().(Collection); ok {
			vm.Push(collection.Spread()...)
		}
	case DESTRUCTURE:
		if pattern, ok := instr.Value.(DestructurePattern); ok {
			if value := vm.Pop(); pattern.Match(value) {
				pattern.Bind(value, vm)
			}
		}
	case COMPREHENSION:
		if comp, ok := instr.Value.(Comprehension); ok {
			vm.Push(comp.Evaluate(vm))
		}
	case PIPELINE:
		if pipeline, ok := instr.Value.(Pipeline); ok {
			vm.Push(pipeline.Execute(vm))
		}
	case CURRY:
		if fn, ok := vm.Pop().(Function); ok {
			vm.Push(fn.Curry())
		}
	case COMPOSE:
		if fns, ok := vm.Pop().([]Function); ok {
			vm.Push(Compose(fns...))
		}
	case MEMOIZE:
		if fn, ok := vm.Pop().(Function); ok {
			vm.Push(fn.Memoize())
		}
	case LAZY:
		if expr, ok := instr.Value.(Expression); ok {
			vm.Push(expr.Lazy())
		}
	case STREAM:
		if stream, ok := instr.Value.(Stream); ok {
			vm.Push(stream)
		}
	case REACTIVE:
		if reactive, ok := instr.Value.(Reactive); ok {
			vm.Push(reactive)
		}
	case TRANSACTION:
		if tx, ok := instr.Value.(Transaction); ok {
			if err := tx.Execute(vm); err != nil {
				return err
			}
		}
	case ATOMIC:
		if atomic, ok := instr.Value.(Atomic); ok {
			if err := atomic.Execute(vm); err != nil {
				return err
			}
		}
	case PARALLEL:
		if parallel, ok := instr.Value.(Parallel); ok {
			if err := parallel.Execute(vm); err != nil {
				return err
			}
		}
	case RACE:
		if race, ok := instr.Value.(Race); ok {
			if err := race.Execute(vm); err != nil {
				return err
			}
		}
	case TIMEOUT:
		if timeout, ok := instr.Value.(Timeout); ok {
			if err := timeout.Execute(vm); err != nil {
				return err
			}
		}
	case RETRY:
		if retry, ok := instr.Value.(Retry); ok {
			if err := retry.Execute(vm); err != nil {
				return err
			}
		}
	case CIRCUIT_BREAKER:
		if cb, ok := instr.Value.(CircuitBreaker); ok {
			if err := cb.Execute(vm); err != nil {
				return err
			}
		}
	case RATE_LIMIT:
		if rl, ok := instr.Value.(RateLimit); ok {
			if err := rl.Execute(vm); err != nil {
				return err
			}
		}
	case BULKHEAD:
		if bh, ok := instr.Value.(Bulkhead); ok {
			if err := bh.Execute(vm); err != nil {
				return err
			}
		}
	case CACHE:
		if cache, ok := instr.Value.(Cache); ok {
			if err := cache.Execute(vm); err != nil {
				return err
			}
		}
	case METRICS:
		if metrics, ok := instr.Value.(Metrics); ok {
			if err := metrics.Execute(vm); err != nil {
				return err
			}
		}
	case TRACING:
		if tracing, ok := instr.Value.(Tracing); ok {
			if err := tracing.Execute(vm); err != nil {
				return err
			}
		}
	case LOGGING:
		if logging, ok := instr.Value.(Logging); ok {
			if err := logging.Execute(vm); err != nil {
				return err
			}
		}
	case PROFILING:
		if profiling, ok := instr.Value.(Profiling); ok {
			if err := profiling.Execute(vm); err != nil {
				return err
			}
		}
	case DEBUGGING:
		if debugging, ok := instr.Value.(Debugging); ok {
			if err := debugging.Execute(vm); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *VM) LoadLib(name string) error {
	switch name {
	case "crypto":
		vm.crypto.RegisterFunctions()
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

func (vm *VM) Parse(code string) ([]Instruction, error) {
	var instructions []Instruction
	lines := strings.Split(code, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		op := parts[0]
		var value interface{}
		if len(parts) > 1 {
			valueStr := strings.Join(parts[1:], " ")
			var err error
			value, err = parseValue(valueStr)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", i+1, err)
			}
		}

		opCode, ok := opCodes[op]
		if !ok {
			return nil, fmt.Errorf("line %d: unknown opcode %s", i+1, op)
		}

		instructions = append(instructions, Instruction{
			Op:    opCode,
			Value: value,
		})
	}

	return instructions, nil
}

func parseValue(valueStr string) (interface{}, error) {
	if valueStr == "nil" {
		return nil, nil
	}

	if valueStr == "true" {
		return true, nil
	}

	if valueStr == "false" {
		return false, nil
	}

	if num, err := strconv.Atoi(valueStr); err == nil {
		return num, nil
	}

	if strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"") {
		return strings.Trim(valueStr, "\""), nil
	}

	if strings.HasPrefix(valueStr, "[") && strings.HasSuffix(valueStr, "]") {
		inner := strings.Trim(valueStr, "[]")
		parts := strings.Split(inner, ",")
		var result []interface{}
		for _, part := range parts {
			val, err := parseValue(strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			result = append(result, val)
		}
		return result, nil
	}

	if strings.HasPrefix(valueStr, "{") && strings.HasSuffix(valueStr, "}") {
		inner := strings.Trim(valueStr, "{}")
		parts := strings.Split(inner, ",")
		result := make(map[string]interface{})
		for _, part := range parts {
			kv := strings.SplitN(part, ":", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid map entry: %s", part)
			}
			key := strings.TrimSpace(kv[0])
			val, err := parseValue(strings.TrimSpace(kv[1]))
			if err != nil {
				return nil, err
			}
			result[key] = val
		}
		return result, nil
	}

	return valueStr, nil
}

var opCodes = map[string]OpCode{
	"PUSH":            PUSH,
	"POP":             POP,
	"ADD":             ADD,
	"SUB":             SUB,
	"MUL":             MUL,
	"DIV":             DIV,
	"MOD":             MOD,
	"LOAD":            LOAD,
	"STORE":           STORE,
	"JMP":             JMP,
	"JMPIF":           JMPIF,
	"JMPIFNOT":        JMPIFNOT,
	"CALL":            CALL,
	"RET":             RET,
	"SPAWN":           SPAWN,
	"YIELD":           YIELD,
	"JOIN":            JOIN,
	"CHANNEL":         CHANNEL,
	"SEND":            SEND,
	"RECV":            RECV,
	"SELECT":          SELECT,
	"TIMER":           TIMER,
	"LIST":            LIST,
	"DICT":            DICT,
	"STRUCT":          STRUCT,
	"STRING":          STRING,
	"APPEND":          APPEND,
	"SET":             SET,
	"FIELD":           FIELD,
	"PERSIST":         PERSIST,
	"LOAD_PERSISTED":  LOAD_PERSISTED,
	"PRINT":           PRINT,
	"READ":            READ,
	"GLOBAL_GET":      GLOBAL_GET,
	"GLOBAL_SET":      GLOBAL_SET,
	"NEW_OBJECT":      NEW_OBJECT,
	"METHOD_CALL":     METHOD_CALL,
	"TRY":             TRY,
	"THROW":           THROW,
	"FILE_OPEN":       FILE_OPEN,
	"FILE_CLOSE":      FILE_CLOSE,
	"FILE_READ":       FILE_READ,
	"FILE_WRITE":      FILE_WRITE,
	"IMPORT":          IMPORT,
	"EXPORT":          EXPORT,
	"DEFER":           DEFER,
	"RECOVER":         RECOVER,
	"RANGE":           RANGE,
	"YIELD_FROM":      YIELD_FROM,
	"AWAIT":           AWAIT,
	"CLASS":           CLASS,
	"INTERFACE":       INTERFACE,
	"GENERIC":         GENERIC,
	"DECORATOR":       DECORATOR,
	"PATTERN_MATCH":   PATTERN_MATCH,
	"SPREAD":          SPREAD,
	"DESTRUCTURE":     DESTRUCTURE,
	"COMPREHENSION":   COMPREHENSION,
	"PIPELINE":        PIPELINE,
	"CURRY":           CURRY,
	"COMPOSE":         COMPOSE,
	"MEMOIZE":         MEMOIZE,
	"LAZY":            LAZY,
	"STREAM":          STREAM,
	"REACTIVE":        REACTIVE,
	"TRANSACTION":     TRANSACTION,
	"ATOMIC":          ATOMIC,
	"PARALLEL":        PARALLEL,
	"RACE":            RACE,
	"TIMEOUT":         TIMEOUT,
	"RETRY":           RETRY,
	"CIRCUIT_BREAKER": CIRCUIT_BREAKER,
	"RATE_LIMIT":      RATE_LIMIT,
	"BULKHEAD":        BULKHEAD,
	"CACHE":           CACHE,
	"METRICS":         METRICS,
	"TRACING":         TRACING,
	"LOGGING":         LOGGING,
	"PROFILING":       PROFILING,
	"DEBUGGING":       DEBUGGING,
}

func (vm *VM) importModule(name string) error {
	if _, exists := vm.imports[name]; exists {
		return nil
	}

	module, err := vm.loadModule(name)
	if err != nil {
		return err
	}

	vm.modules[name] = module
	vm.imports[name] = true
	return nil
}

func (vm *VM) loadModule(name string) (*Module, error) {
	// Check if module is already loaded
	if module, exists := vm.modules[name]; exists {
		return module, nil
	}

	// Create new module
	module := &Module{
		Name:      name,
		Functions: make(map[string][]Instruction),
		Globals:   make(map[string]Value),
		Types:     make(map[string]*TypeInfo),
		Imports:   make(map[string]bool),
		Exports:   make(map[string]bool),
	}

	// Try to load from disk first
	modulePath := fmt.Sprintf("./modules/%s.cvm", name)
	if _, err := os.Stat(modulePath); err == nil {
		// Read module file
		content, err := os.ReadFile(modulePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read module file: %v", err)
		}

		// Parse module content
		instructions, err := vm.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse module: %v", err)
		}

		// Execute module initialization
		if err := vm.executeModule(module, instructions); err != nil {
			return nil, fmt.Errorf("failed to initialize module: %v", err)
		}

		return module, nil
	}

	// Try to load from network if not found on disk
	moduleURL := fmt.Sprintf("https://modules.cvm.io/%s", name)
	resp, err := http.Get(moduleURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch module from network: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("module not found on network: %s", name)
	}

	// Read module content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read module content: %v", err)
	}

	// Parse module content
	instructions, err := vm.Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse module: %v", err)
	}

	// Execute module initialization
	if err := vm.executeModule(module, instructions); err != nil {
		return nil, fmt.Errorf("failed to initialize module: %v", err)
	}

	// Cache module locally
	if err := os.MkdirAll("./modules", 0755); err != nil {
		return nil, fmt.Errorf("failed to create modules directory: %v", err)
	}

	if err := os.WriteFile(modulePath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to cache module: %v", err)
	}

	return module, nil
}

func (vm *VM) executeModule(module *Module, instructions []Instruction) error {
	// Save current VM state
	savedStack := vm.stack
	savedPC := vm.pc
	savedCode := vm.code
	savedGlobals := vm.globals
	savedTypes := vm.types

	// Set up module execution environment
	vm.stack = make([]Value, 0, 4096)
	vm.pc = 0
	vm.code = instructions
	vm.globals = module.Globals
	vm.types = module.Types

	// Execute module code
	err := vm.Execute(instructions)

	// Restore VM state
	vm.stack = savedStack
	vm.pc = savedPC
	vm.code = savedCode
	vm.globals = savedGlobals
	vm.types = savedTypes

	if err != nil {
		return fmt.Errorf("module execution failed: %v", err)
	}

	// Copy exported functions and types
	for name, fn := range vm.functions {
		if module.Exports[name] {
			module.Functions[name] = fn
		}
	}

	return nil
}

func (vm *VM) defineClass(info map[string]interface{}) {
	name := info["name"].(string)
	baseType := info["baseType"].(string)
	interfaces := info["interfaces"].([]string)
	generics := info["generics"].([]string)

	vm.types[name] = &TypeInfo{
		Name:       name,
		BaseType:   baseType,
		Interfaces: interfaces,
		Generics:   generics,
		Properties: make(map[string]PropertyInfo),
		Methods:    make(map[string]MethodInfo),
	}
}

func (vm *VM) defineInterface(info map[string]interface{}) {
	name := info["name"].(string)
	methods := info["methods"].(map[string]MethodInfo)

	vm.types[name] = &TypeInfo{
		Name:       name,
		Methods:    methods,
		Properties: make(map[string]PropertyInfo),
	}
}

func (vm *VM) defineGeneric(info map[string]interface{}) {
	name := info["name"].(string)
	constraints := info["constraints"].([]string)

	vm.types[name] = &TypeInfo{
		Name:       name,
		Generics:   constraints,
		Properties: make(map[string]PropertyInfo),
		Methods:    make(map[string]MethodInfo),
	}
}

func (vm *VM) applyDecorator(info map[string]interface{}) {
	target := info["target"].(string)
	decorator := info["decorator"].(string)
	args := info["args"].([]interface{})

	if fn, exists := vm.functions[target]; exists {
		vm.functions[target] = vm.decorateFunction(fn, decorator, args)
	}
}

func (vm *VM) decorateFunction(fn []Instruction, decorator string, args []interface{}) []Instruction {
	// Create wrapper instructions for the decorator
	wrapper := make([]Instruction, 0)

	// Push decorator arguments
	for _, arg := range args {
		wrapper = append(wrapper, Instruction{Op: PUSH, Value: arg})
	}

	// Push the original function
	wrapper = append(wrapper, Instruction{Op: PUSH, Value: fn})

	// Add decorator-specific instructions
	switch decorator {
	case "memoize":
		// Add memoization wrapper
		wrapper = append(wrapper, []Instruction{
			{Op: PUSH, Value: make(map[interface{}]interface{})}, // Cache map
			{Op: PUSH, Value: fn},                                // Original function
			{Op: PUSH, Value: "memoize_wrapper"},                 // Wrapper name
			{Op: CALL, Value: "memoize_wrapper"},                 // Call wrapper
		}...)

	case "timing":
		// Add timing wrapper
		wrapper = append(wrapper, []Instruction{
			{Op: PUSH, Value: "start_time"},     // Start time label
			{Op: CALL, Value: "time_now"},       // Get current time
			{Op: PUSH, Value: fn},               // Original function
			{Op: PUSH, Value: "timing_wrapper"}, // Wrapper name
			{Op: CALL, Value: "timing_wrapper"}, // Call wrapper
		}...)

	case "retry":
		// Add retry wrapper
		maxRetries := 3
		if len(args) > 0 {
			if retries, ok := args[0].(int); ok {
				maxRetries = retries
			}
		}
		wrapper = append(wrapper, []Instruction{
			{Op: PUSH, Value: maxRetries},      // Max retries
			{Op: PUSH, Value: fn},              // Original function
			{Op: PUSH, Value: "retry_wrapper"}, // Wrapper name
			{Op: CALL, Value: "retry_wrapper"}, // Call wrapper
		}...)

	case "logging":
		// Add logging wrapper
		wrapper = append(wrapper, []Instruction{
			{Op: PUSH, Value: fn},                // Original function
			{Op: PUSH, Value: "logging_wrapper"}, // Wrapper name
			{Op: CALL, Value: "logging_wrapper"}, // Call wrapper
		}...)

	case "validation":
		// Add validation wrapper
		if len(args) > 0 {
			validator := args[0]
			wrapper = append(wrapper, []Instruction{
				{Op: PUSH, Value: validator},            // Validator function
				{Op: PUSH, Value: fn},                   // Original function
				{Op: PUSH, Value: "validation_wrapper"}, // Wrapper name
				{Op: CALL, Value: "validation_wrapper"}, // Call wrapper
			}...)
		}

	case "caching":
		// Add caching wrapper
		ttl := 3600 // Default TTL in seconds
		if len(args) > 0 {
			if cacheTTL, ok := args[0].(int); ok {
				ttl = cacheTTL
			}
		}
		wrapper = append(wrapper, []Instruction{
			{Op: PUSH, Value: ttl},               // Cache TTL
			{Op: PUSH, Value: fn},                // Original function
			{Op: PUSH, Value: "caching_wrapper"}, // Wrapper name
			{Op: CALL, Value: "caching_wrapper"}, // Call wrapper
		}...)

	default:
		// For unknown decorators, just return the original function
		return fn
	}

	// Add return instruction
	wrapper = append(wrapper, Instruction{Op: RET, Value: nil})

	return wrapper
}

func (g *Generator) Next() (interface{}, bool) {
	if g.finished {
		return nil, true
	}

	// Execute until we hit a YIELD instruction or finish
	for g.pc < len(g.code) {
		instr := g.code[g.pc]
		g.pc++

		switch instr.Op {
		case YIELD:
			// Return the yielded value
			if len(g.stack) > 0 {
				value := g.stack[len(g.stack)-1]
				g.stack = g.stack[:len(g.stack)-1]
				return value, false
			}
			return nil, false

		case YIELD_FROM:
			// Handle nested generators
			if gen, ok := instr.Value.(*Generator); ok {
				if value, done := gen.Next(); !done {
					return value, false
				}
			}

		case RET:
			// Generator finished
			g.finished = true
			if len(g.stack) > 0 {
				value := g.stack[len(g.stack)-1]
				g.stack = g.stack[:len(g.stack)-1]
				return value, true
			}
			return nil, true

		case PUSH:
			g.stack = append(g.stack, instr.Value)

		case POP:
			if len(g.stack) > 0 {
				g.stack = g.stack[:len(g.stack)-1]
			}

		case LOAD:
			if name, ok := instr.Value.(string); ok {
				if value, exists := g.locals[name]; exists {
					g.stack = append(g.stack, value)
				}
			}

		case STORE:
			if name, ok := instr.Value.(string); ok {
				if len(g.stack) > 0 {
					g.locals[name] = g.stack[len(g.stack)-1]
					g.stack = g.stack[:len(g.stack)-1]
				}
			}

		case CALL:
			if name, ok := instr.Value.(string); ok {
				// Handle built-in functions
				switch name {
				case "range":
					if len(g.stack) >= 2 {
						end := g.stack[len(g.stack)-1].(int)
						start := g.stack[len(g.stack)-2].(int)
						g.stack = g.stack[:len(g.stack)-2]
						if start < end {
							g.stack = append(g.stack, start+1)
							return start, false
						}
						g.finished = true
						return nil, true
					}
				case "enumerate":
					if len(g.stack) > 0 {
						iter := g.stack[len(g.stack)-1]
						g.stack = g.stack[:len(g.stack)-1]
						if iterator, ok := iter.(Iterator); ok {
							if iterator.HasNext() {
								value := iterator.Next()
								g.stack = append(g.stack, value)
								return value, false
							}
						}
						g.finished = true
						return nil, true
					}
				}
			}

		case JMP:
			if addr, ok := instr.Value.(int); ok {
				g.pc = addr
			}

		case JMPIF:
			if addr, ok := instr.Value.(int); ok {
				if len(g.stack) > 0 {
					cond := g.stack[len(g.stack)-1]
					g.stack = g.stack[:len(g.stack)-1]
					if cond != nil && cond != false {
						g.pc = addr
					}
				}
			}

		case JMPIFNOT:
			if addr, ok := instr.Value.(int); ok {
				if len(g.stack) > 0 {
					cond := g.stack[len(g.stack)-1]
					g.stack = g.stack[:len(g.stack)-1]
					if cond == nil || cond == false {
						g.pc = addr
					}
				}
			}
		}
	}

	// If we get here, the generator is finished
	g.finished = true
	return nil, true
}

func (p *Promise) Await() (interface{}, error) {
	<-p.done
	return p.value, p.err
}

func Compose(fns ...Function) Function {
	return &composedFunction{functions: fns}
}

type composedFunction struct {
	functions []Function
}

func (cf *composedFunction) Call(args ...interface{}) interface{} {
	result := args[0]
	for _, fn := range cf.functions {
		result = fn.Call(result)
	}
	return result
}

func (cf *composedFunction) Curry() Function {
	return cf
}

func (cf *composedFunction) Memoize() Function {
	return cf
}
