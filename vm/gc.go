package vm

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"
)

type ObjectType int

const (
	TypeInt ObjectType = iota
	TypeFloat
	TypeString
	TypeList
	TypeDict
	TypeStruct
	TypeChannel
	TypeRoutine
)

type Generation int

const (
	Young Generation = iota
	Old
)

type Object struct {
	Type      ObjectType
	Value     interface{}
	RefCount  int
	LastUsed  time.Time
	Persisted bool
	Metadata  map[string]interface{}
	Gen       Generation
	Age       int
}

type GC struct {
	objects     map[uint64]*Object
	nextID      uint64
	mutex       sync.RWMutex
	persistPath string
	threshold   int
	interval    time.Duration
	memManager  *MemoryManager
	youngGen    []uint64
	oldGen      []uint64
	maxAge      int
}

func NewGC(persistPath string) *GC {
	gc := &GC{
		objects:     make(map[uint64]*Object),
		persistPath: persistPath,
		threshold:   1000,
		interval:    time.Minute * 5,
		memManager:  NewMemoryManager(),
		youngGen:    make([]uint64, 0, 1024),
		oldGen:      make([]uint64, 0, 1024),
		maxAge:      3,
	}
	go gc.startBackgroundGC()
	return gc
}

func (gc *GC) Allocate(objType ObjectType, value interface{}) uint64 {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	id := gc.nextID
	gc.nextID++

	obj := &Object{
		Type:      objType,
		Value:     value,
		RefCount:  1,
		LastUsed:  time.Now(),
		Persisted: false,
		Metadata:  make(map[string]interface{}),
		Gen:       Young,
		Age:       0,
	}

	gc.objects[id] = obj
	gc.youngGen = append(gc.youngGen, id)
	return id
}

func (gc *GC) IncrementRef(id uint64) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	if obj, exists := gc.objects[id]; exists {
		obj.RefCount++
		obj.LastUsed = time.Now()
	}
}

func (gc *GC) DecrementRef(id uint64) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	if obj, exists := gc.objects[id]; exists {
		obj.RefCount--
		if obj.RefCount <= 0 {
			if obj.Gen == Young {
				gc.removeFromGen(gc.youngGen, id)
			} else {
				gc.removeFromGen(gc.oldGen, id)
			}
			delete(gc.objects, id)
		}
	}
}

func (gc *GC) removeFromGen(gen []uint64, id uint64) {
	for i, objID := range gen {
		if objID == id {
			gen[i] = gen[len(gen)-1]
			gen = gen[:len(gen)-1]
			break
		}
	}
}

func (gc *GC) promoteObject(id uint64) {
	obj := gc.objects[id]
	obj.Gen = Old
	gc.removeFromGen(gc.youngGen, id)
	gc.oldGen = append(gc.oldGen, id)
}

func (gc *GC) collectYoungGen() {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	for _, id := range gc.youngGen {
		obj := gc.objects[id]
		if obj.RefCount <= 0 {
			delete(gc.objects, id)
		} else {
			obj.Age++
			if obj.Age >= gc.maxAge {
				gc.promoteObject(id)
			}
		}
	}
}

func (gc *GC) collectOldGen() {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	for _, id := range gc.oldGen {
		obj := gc.objects[id]
		if obj.RefCount <= 0 {
			delete(gc.objects, id)
			gc.removeFromGen(gc.oldGen, id)
		}
	}
}

func (gc *GC) startBackgroundGC() {
	ticker := time.NewTicker(gc.interval)
	for range ticker.C {
		gc.collectYoungGen()
		if len(gc.objects) > gc.threshold {
			gc.collectOldGen()
		}
	}
}

func (gc *GC) Get(id uint64) *Object {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()

	if obj, exists := gc.objects[id]; exists {
		obj.LastUsed = time.Now()
		return obj
	}
	return nil
}

func (gc *GC) Persist(id uint64) error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	obj, exists := gc.objects[id]
	if !exists {
		return nil
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	filename := gc.persistPath + "/" + strconv.FormatUint(id, 10) + ".json"
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	obj.Persisted = true
	return nil
}

func (gc *GC) Load(id uint64) error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	filename := gc.persistPath + "/" + strconv.FormatUint(id, 10) + ".json"
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var obj Object
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return err
	}

	gc.objects[id] = &obj
	return nil
}

func (gc *GC) CreateList() uint64 {
	return gc.Allocate(TypeList, make([]interface{}, 0))
}

func (gc *GC) CreateDict() uint64 {
	return gc.Allocate(TypeDict, make(map[string]interface{}))
}

func (gc *GC) CreateStruct(fields map[string]interface{}) uint64 {
	return gc.Allocate(TypeStruct, fields)
}

func (gc *GC) CreateString(value string) uint64 {
	return gc.Allocate(TypeString, value)
}

func (gc *GC) ListAppend(listID uint64, value interface{}) {
	if obj := gc.Get(listID); obj != nil && obj.Type == TypeList {
		if list, ok := obj.Value.([]interface{}); ok {
			obj.Value = append(list, value)
		}
	}
}

func (gc *GC) DictSet(dictID uint64, key string, value interface{}) {
	if obj := gc.Get(dictID); obj != nil && obj.Type == TypeDict {
		if dict, ok := obj.Value.(map[string]interface{}); ok {
			dict[key] = value
		}
	}
}

func (gc *GC) StructSet(structID uint64, field string, value interface{}) {
	if obj := gc.Get(structID); obj != nil && obj.Type == TypeStruct {
		if fields, ok := obj.Value.(map[string]interface{}); ok {
			fields[field] = value
		}
	}
}
