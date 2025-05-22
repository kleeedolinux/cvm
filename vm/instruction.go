package vm

type OpCode byte

const (
	// Stack operations
	PUSH OpCode = iota
	POP
	DUP
	SWAP

	// Arithmetic operations
	ADD
	SUB
	MUL
	DIV
	MOD

	// Comparison operations
	EQ
	NEQ
	GT
	LT
	GTE
	LTE

	// Control flow
	JMP
	JMPIF
	JMPIFNOT
	CALL
	RET

	// Memory operations
	LOAD
	STORE

	// Concurrency operations
	SPAWN
	YIELD
	JOIN
	CHANNEL
	SEND
	RECV
	SELECT
	TIMER

	// Complex data structures
	LIST
	DICT
	STRUCT
	STRING
	APPEND
	SET
	FIELD
	PERSIST
	LOAD_PERSISTED
)

type Instruction struct {
	Op    OpCode
	Value interface{}
}
