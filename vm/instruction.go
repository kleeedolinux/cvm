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
	PRINTF
	SCANF
	WRITE
	READ_BYTES
	WRITE_BYTES
	FLUSH
	SEEK
	TRUNCATE
	SYNC
	BUFFER
	UNBUFFER

	// Network operations
	SOCKET
	BIND
	LISTEN
	ACCEPT
	CONNECT
	SEND_TO
	RECV_FROM
	CLOSE_SOCKET

	// High-performance I/O
	MMAP
	MUNMAP
	DIRECT_IO
	BUFFERED_IO
	ASYNC_IO
	POLL
	EPOLL
	SELECT_IO
	KQUEUE

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
	FILE_STAT
	FILE_CHMOD
	FILE_CHOWN
	FILE_LINK
	FILE_SYMLINK
	FILE_READLINK
	FILE_REMOVE
	FILE_RENAME
	FILE_MKDIR
	FILE_RMDIR
	FILE_READDIR
	FILE_ACCESS
	FILE_EXISTS
	FILE_IS_DIR
	FILE_IS_FILE
	FILE_SIZE
	FILE_MTIME
	FILE_ATIME
	FILE_CTIME
	FILE_PERM
	FILE_OWNER
	FILE_GROUP

	// Module system
	IMPORT
	EXPORT
	MODULE_GET
	MODULE_SET

	// Advanced control flow
	DEFER
	RECOVER
	RANGE
	YIELD_FROM
	AWAIT

	// Object-oriented features
	CLASS
	INTERFACE
	GENERIC
	DECORATOR
	INHERIT
	IMPLEMENT
	OVERRIDE
	SUPER
	THIS

	// Pattern matching and destructuring
	PATTERN_MATCH
	SPREAD
	DESTRUCTURE
	COMPREHENSION
	PIPELINE

	// Functional programming
	CURRY
	COMPOSE
	MEMOIZE
	LAZY
	PARTIAL
	APPLY
	BIND_FN

	// Reactive programming
	STREAM
	REACTIVE
	OBSERVABLE
	SUBSCRIBE
	PUBLISH
	FILTER
	MAP
	REDUCE

	// Concurrency patterns
	TRANSACTION
	ATOMIC
	PARALLEL
	RACE
	TIMEOUT
	RETRY
	CIRCUIT_BREAKER
	RATE_LIMIT
	BULKHEAD

	// Caching and performance
	CACHE
	MEMOIZE_FN
	LAZY_EVAL
	PRELOAD
	PREFETCH

	// Observability
	METRICS
	TRACING
	LOGGING
	PROFILING
	DEBUGGING
	TELEMETRY
	MONITORING
	ALERTING
)

type Instruction struct {
	Op    OpCode
	Value interface{}
}

// Core interfaces for advanced features
type Iterator interface {
	HasNext() bool
	Next() interface{}
}

type Generator struct {
	code     []Instruction
	pc       int
	stack    []Value
	locals   map[string]Value
	finished bool
}

type Promise struct {
	value interface{}
	err   error
	done  chan struct{}
}

type Pattern interface {
	Match(value interface{}) bool
	Extract(value interface{}) interface{}
}

type Collection interface {
	Spread() []interface{}
}

type DestructurePattern interface {
	Match(value interface{}) bool
	Bind(value interface{}, vm *VM)
}

type Comprehension interface {
	Evaluate(vm *VM) interface{}
}

type Pipeline interface {
	Execute(vm *VM) interface{}
}

type Function interface {
	Call(args ...interface{}) interface{}
	Curry() Function
	Memoize() Function
}

type Stream interface {
	Next() (interface{}, bool)
	Close()
}

type Reactive interface {
	Subscribe(observer interface{})
	Unsubscribe(observer interface{})
	Notify(value interface{})
}

type Transaction interface {
	Execute(vm *VM) error
	Commit() error
	Rollback() error
}

type Atomic interface {
	Execute(vm *VM) error
}

type Parallel interface {
	Execute(vm *VM) error
}

type Race interface {
	Execute(vm *VM) error
}

type Timeout interface {
	Execute(vm *VM) error
}

type Retry interface {
	Execute(vm *VM) error
}

type CircuitBreaker interface {
	Execute(vm *VM) error
}

type RateLimit interface {
	Execute(vm *VM) error
}

type Bulkhead interface {
	Execute(vm *VM) error
}

type Cache interface {
	Execute(vm *VM) error
}

type Metrics interface {
	Execute(vm *VM) error
}

type Tracing interface {
	Execute(vm *VM) error
}

type Logging interface {
	Execute(vm *VM) error
}

type Profiling interface {
	Execute(vm *VM) error
}

type Debugging interface {
	Execute(vm *VM) error
}

type Expression interface {
	Evaluate(vm *VM) interface{}
	Lazy() Expression
}
