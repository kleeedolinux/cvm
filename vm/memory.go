package vm

import (
	"errors"
	"sync"
	"unsafe"
)

const (
	PageSize     = 4096
	MinBlockSize = 16
	MaxBlockSize = PageSize
	IOBufferSize = 8192
	MMapSize     = 1024 * 1024
)

type MemoryBlock struct {
	start    uintptr
	size     uintptr
	used     bool
	next     *MemoryBlock
	prev     *MemoryBlock
	refCount int
	ioBuffer []byte
	mmapped  bool
}

type MemoryPool struct {
	blocks    []*MemoryBlock
	mutex     sync.RWMutex
	pageCount int
	ioPools   map[int]*MemoryPool
	objPool   *sync.Pool
}

type MemoryManager struct {
	pools     map[uintptr]*MemoryPool
	mutex     sync.RWMutex
	alignment uintptr
	ioCache   *sync.Pool
}

func NewMemoryManager() *MemoryManager {
	mm := &MemoryManager{
		pools:     make(map[uintptr]*MemoryPool),
		alignment: unsafe.Alignof(uint64(0)),
		ioCache: &sync.Pool{
			New: func() interface{} {
				return make([]byte, IOBufferSize)
			},
		},
	}
	return mm
}

func (mm *MemoryManager) allocate(size uintptr) (uintptr, error) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	alignedSize := (size + mm.alignment - 1) & ^(mm.alignment - 1)
	pool := mm.getOrCreatePool(alignedSize)

	block := pool.allocateBlock(alignedSize)
	if block == nil {
		return 0, ErrOutOfMemory
	}

	return block.start, nil
}

func (mm *MemoryManager) free(ptr uintptr) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	for _, pool := range mm.pools {
		if block := pool.findBlock(ptr); block != nil {
			pool.freeBlock(block)
			return
		}
	}
}

func (mm *MemoryManager) getOrCreatePool(size uintptr) *MemoryPool {
	if size < MinBlockSize {
		size = MinBlockSize
	}
	if size > MaxBlockSize {
		size = MaxBlockSize
	}

	if pool, exists := mm.pools[size]; exists {
		return pool
	}

	pool := &MemoryPool{
		blocks:    make([]*MemoryBlock, 0),
		pageCount: 1,
		ioPools:   make(map[int]*MemoryPool),
		objPool: &sync.Pool{
			New: func() interface{} {
				return &MemoryBlock{
					used:     false,
					refCount: 0,
					ioBuffer: make([]byte, IOBufferSize),
				}
			},
		},
	}
	mm.pools[size] = pool
	return pool
}

func (mp *MemoryPool) getBlock() *MemoryBlock {
	return mp.objPool.Get().(*MemoryBlock)
}

func (mp *MemoryPool) putBlock(block *MemoryBlock) {
	block.used = false
	block.refCount = 0
	mp.objPool.Put(block)
}

func (mp *MemoryPool) allocateBlock(size uintptr) *MemoryBlock {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	for _, block := range mp.blocks {
		if !block.used {
			block.used = true
			block.refCount = 1
			return block
		}
	}

	block := mp.getBlock()
	block.start = uintptr(unsafe.Pointer(&make([]byte, size)[0]))
	block.size = size
	block.used = true
	block.refCount = 1

	mp.blocks = append(mp.blocks, block)
	return block
}

func (mp *MemoryPool) freeBlock(block *MemoryBlock) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	block.refCount--
	if block.refCount <= 0 {
		mp.putBlock(block)
		mp.defragment()
	}
}

func (mp *MemoryPool) findBlock(ptr uintptr) *MemoryBlock {
	for _, block := range mp.blocks {
		if block.start == ptr {
			return block
		}
	}
	return nil
}

func (mp *MemoryPool) defragment() {
	var newBlocks []*MemoryBlock
	var currentBlock *MemoryBlock

	for _, block := range mp.blocks {
		if !block.used {
			if currentBlock == nil {
				currentBlock = block
			} else {
				currentBlock.size += block.size
			}
		} else {
			if currentBlock != nil {
				newBlocks = append(newBlocks, currentBlock)
				currentBlock = nil
			}
			newBlocks = append(newBlocks, block)
		}
	}

	if currentBlock != nil {
		newBlocks = append(newBlocks, currentBlock)
	}

	mp.blocks = newBlocks
}

func (mm *MemoryManager) allocateIOBuffer() []byte {
	return mm.ioCache.Get().([]byte)
}

func (mm *MemoryManager) freeIOBuffer(buf []byte) {
	mm.ioCache.Put(buf)
}

func (mm *MemoryManager) allocateMMap(size uintptr) (uintptr, error) {
	if size > MMapSize {
		size = MMapSize
	}

	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	pool := mm.getOrCreatePool(size)
	block := pool.allocateBlock(size)
	if block == nil {
		return 0, ErrOutOfMemory
	}

	block.mmapped = true
	block.ioBuffer = make([]byte, size)
	return block.start, nil
}

func (mm *MemoryManager) freeMMap(ptr uintptr) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	for _, pool := range mm.pools {
		if block := pool.findBlock(ptr); block != nil && block.mmapped {
			block.ioBuffer = nil
			pool.freeBlock(block)
			return
		}
	}
}

func (mp *MemoryPool) allocateIOBlock(size uintptr) *MemoryBlock {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	if size < IOBufferSize {
		size = IOBufferSize
	}

	block := &MemoryBlock{
		start:    uintptr(unsafe.Pointer(&make([]byte, size)[0])),
		size:     size,
		used:     true,
		refCount: 1,
		ioBuffer: make([]byte, size),
	}

	mp.blocks = append(mp.blocks, block)
	return block
}

var ErrOutOfMemory = errors.New("out of memory")
