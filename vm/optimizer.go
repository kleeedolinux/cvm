package vm

import (
	"strconv"
	"sync"
)

type Optimizer struct {
	cache    map[string][]Instruction
	mutex    sync.RWMutex
	maxCache int
}

func NewOptimizer(maxCache int) *Optimizer {
	return &Optimizer{
		cache:    make(map[string][]Instruction),
		maxCache: maxCache,
	}
}

func (o *Optimizer) Optimize(code []Instruction) []Instruction {
	o.mutex.RLock()
	if cached, ok := o.cache[hashInstructions(code)]; ok {
		o.mutex.RUnlock()
		return cached
	}
	o.mutex.RUnlock()

	optimized := o.optimizeConstantFolding(code)
	optimized = o.optimizeDeadCodeElimination(optimized)
	optimized = o.optimizePeephole(optimized)
	optimized = o.optimizeIOOperations(optimized)
	optimized = o.optimizeNetworkOperations(optimized)

	o.mutex.Lock()
	if len(o.cache) >= o.maxCache {
		o.cache = make(map[string][]Instruction)
	}
	o.cache[hashInstructions(code)] = optimized
	o.mutex.Unlock()

	return optimized
}

func (o *Optimizer) optimizeConstantFolding(code []Instruction) []Instruction {
	var result []Instruction
	for i := 0; i < len(code); i++ {
		if i+2 < len(code) {
			if isConstantOperation(code[i : i+3]) {
				result = append(result, foldConstants(code[i:i+3])...)
				i += 2
				continue
			}
		}
		result = append(result, code[i])
	}
	return result
}

func (o *Optimizer) optimizeDeadCodeElimination(code []Instruction) []Instruction {
	var result []Instruction
	for i := 0; i < len(code); i++ {
		if isDeadCode(code[i:]) {
			continue
		}
		result = append(result, code[i])
	}
	return result
}

func (o *Optimizer) optimizePeephole(code []Instruction) []Instruction {
	var result []Instruction
	for i := 0; i < len(code); i++ {
		if i+1 < len(code) {
			if isPeepholeOptimizable(code[i : i+2]) {
				result = append(result, peepholeOptimize(code[i:i+2])...)
				i++
				continue
			}
		}
		result = append(result, code[i])
	}
	return result
}

func isConstantOperation(instrs []Instruction) bool {
	if len(instrs) != 3 {
		return false
	}
	return instrs[0].Op == PUSH && instrs[1].Op == PUSH &&
		(instrs[2].Op == ADD || instrs[2].Op == SUB || instrs[2].Op == MUL || instrs[2].Op == DIV)
}

func foldConstants(instrs []Instruction) []Instruction {
	a, ok1 := instrs[0].Value.(int)
	b, ok2 := instrs[1].Value.(int)
	if !ok1 || !ok2 {
		return instrs
	}

	var result int
	switch instrs[2].Op {
	case ADD:
		result = a + b
	case SUB:
		result = a - b
	case MUL:
		result = a * b
	case DIV:
		if b != 0 {
			result = a / b
		} else {
			return instrs
		}
	default:
		return instrs
	}

	return []Instruction{{Op: PUSH, Value: result}}
}

func isDeadCode(instrs []Instruction) bool {
	if len(instrs) == 0 {
		return false
	}
	return instrs[0].Op == POP && len(instrs) > 1 && instrs[1].Op == POP
}

func isPeepholeOptimizable(instrs []Instruction) bool {
	if len(instrs) != 2 {
		return false
	}
	return (instrs[0].Op == PUSH && instrs[0].Value == 0 && instrs[1].Op == ADD) ||
		(instrs[0].Op == PUSH && instrs[0].Value == 1 && instrs[1].Op == MUL)
}

func peepholeOptimize(instrs []Instruction) []Instruction {
	if instrs[0].Op == PUSH {
		if instrs[0].Value == 0 && instrs[1].Op == ADD {
			return []Instruction{instrs[1]}
		}
		if instrs[0].Value == 1 && instrs[1].Op == MUL {
			return []Instruction{instrs[1]}
		}
	}
	return instrs
}

func hashInstructions(code []Instruction) string {
	var hash string
	for _, instr := range code {
		hash += string(instr.Op)
		if instr.Value != nil {
			if intVal, ok := instr.Value.(int); ok {
				hash += ":" + strconv.Itoa(intVal)
			}
		}
	}
	return hash
}

func (o *Optimizer) optimizeIOOperations(code []Instruction) []Instruction {
	var result []Instruction
	for i := 0; i < len(code); i++ {
		if i+1 < len(code) {
			switch code[i].Op {
			case WRITE_BYTES:
				if code[i+1].Op == FLUSH {
					result = append(result, Instruction{Op: SYNC, Value: code[i].Value})
					i++
					continue
				}
			case READ_BYTES:
				if i+2 < len(code) && code[i+1].Op == BUFFER && code[i+2].Op == READ_BYTES {
					result = append(result, Instruction{Op: BUFFERED_IO, Value: []interface{}{code[i].Value, code[i+2].Value}})
					i += 2
					continue
				}
			case FILE_READ:
				if i+1 < len(code) && code[i+1].Op == BUFFER {
					result = append(result, Instruction{Op: MMAP, Value: code[i].Value})
					i++
					continue
				}
			case FILE_WRITE:
				if i+1 < len(code) && code[i+1].Op == SYNC {
					result = append(result, Instruction{Op: DIRECT_IO, Value: code[i].Value})
					i++
					continue
				}
			}
		}
		result = append(result, code[i])
	}
	return result
}

func (o *Optimizer) optimizeNetworkOperations(code []Instruction) []Instruction {
	var result []Instruction
	for i := 0; i < len(code); i++ {
		if i+2 < len(code) {
			switch code[i].Op {
			case SOCKET:
				if code[i+1].Op == BIND && code[i+2].Op == LISTEN {
					result = append(result, Instruction{Op: LISTEN, Value: []interface{}{code[i].Value, code[i+1].Value}})
					i += 2
					continue
				}
			case SEND_TO:
				if i+1 < len(code) && code[i+1].Op == RECV_FROM {
					result = append(result, Instruction{Op: ASYNC_IO, Value: []interface{}{code[i].Value, code[i+1].Value}})
					i++
					continue
				}
			}
		}
		result = append(result, code[i])
	}
	return result
}
