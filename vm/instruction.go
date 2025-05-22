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

	// I/O operations
	PRINT
	READ

	// Global variables
	GLOBAL_GET
	GLOBAL_SET

	// Object-oriented features
	NEW_OBJECT
	METHOD_CALL

	// Error handling
	TRY
	THROW

	// File operations
	FILE_OPEN
	FILE_CLOSE
	FILE_READ
	FILE_WRITE
)

type Instruction struct {
	Op    OpCode
	Value interface{}
}
