package vm

import (
	"runtime"
	"sync"
	"time"
)

type Value interface{}

type VM struct {
	stack     []Value
	memory    map[int]Value
	pc        int
	code      []Instruction
	mutex     sync.Mutex
	routines  map[int]*VM
	routineID int
	optimizer *Optimizer
	channels  map[int]*Channel
	channelID int
	gc        *GC
}

func NewVM() *VM {
	return &VM{
		stack:     make([]Value, 0, 1024),
		memory:    make(map[int]Value),
		routines:  make(map[int]*VM),
		channels:  make(map[int]*Channel),
		optimizer: NewOptimizer(1000),
		gc:        NewGC("./persist"),
	}
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

func (vm *VM) Execute(code []Instruction) {
	vm.code = vm.optimizer.Optimize(code)
	vm.pc = 0

	for vm.pc < len(vm.code) {
		instr := vm.code[vm.pc]
		vm.executeInstruction(instr)
		vm.pc++
	}
}

func (vm *VM) executeInstruction(instr Instruction) {
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
				if bInt != 0 {
					vm.Push(aInt / bInt)
				}
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
	}
}
