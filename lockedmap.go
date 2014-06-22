package tsmap

import (
	"sync"
)

//struct to hold each element in the map
// it holds the raw data + a pending access queue
type ElementHolder struct {
	element Element
	lockedTransactionId int //emtpy string if not locked, otherwise holds a unique id to prevent
							// someone who doesn't own the element returning it
	locked bool

	lockQueue chan int // callers have to receive an element from the queue before they access the element
}

type ElementLoan struct {
	element Element
	transactionId int
}
func (el *ElementLoan) Element() Element {
	return el.element
}

// a map implementation that is thread safe.
// not only is adding and deleting thread safe but also read-modify-write of elements
//
// elements must conform to Element. That is, they return a key
// the caller has to call Lock to obtain an element. When finished, Unlock must be called
type LockedMap struct {
	m map[string]*ElementHolder
	mutex sync.Mutex // global access mutex
}

// returns a new LockedMap (implements Map), ready to use
func CreateLockedMap() *LockedMap {
	m := new(LockedMap)
	m.m = make(map[string]*ElementHolder)
	// mutex news as unlocked
	return m
}

func (m *LockedMap) Add(element Element) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := element.Key()
	// check if the key already exists
	_,exists :=m.m[key]
	if exists {
		return false
	}

	// create an element holder
	eh :=new(ElementHolder)
	eh.element=element //copy of the interface
	eh.lockQueue=make(chan int,1) //TODO allow customisation of the queue length
	eh.lockQueue <- 1 // add an item so that the first requester can take from the queue
	eh.locked=false

	m.m[key]=eh

	return true
}

func (m *LockedMap) Lock(key string) (MapElementAccess) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	eltHolder,exists := m.m[key]
	if !exists {
		return nil
	}

	// wait on the queue to get access to the element
	m.mutex.Unlock() // need to unlock the global mutex so that the caller calling unlock can get through
	token, ok := <- eltHolder.lockQueue // if we crash here, the defered Unlock will cause a panic
	m.mutex.Lock()
	if !ok { // wait for the lock again to do this is sub-optimal. Replace defer with a good old fashioned unlock everywhere?
		return nil
	}

	token++
	eltHolder.lockedTransactionId=token
	eltHolder.locked = true

	loan := &ElementLoan {
		element: eltHolder.element,
		transactionId: eltHolder.lockedTransactionId,
	}

	return loan
}


func (m *LockedMap) Unlock(eltAccess MapElementAccess) bool {
	loan := eltAccess.(*ElementLoan)
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// check that the loan returned is valid (i.e. it exists in the map)
	eltHolder,exists := m.m[loan.element.Key()]
	if !exists {
		return false
	}

	if eltHolder.locked == false {
		// its not even locked
		return false
	}

	if eltHolder.lockedTransactionId == loan.transactionId {
		// valid
		eltHolder.locked = false // clear the transaction id
		eltHolder.lockQueue <- eltHolder.lockedTransactionId // allow another consumer to unblock and acquire

		loan.element=nil // reset the loan
		return true
	} else {
		return false
	}

}

func (m *LockedMap) Delete(key string) bool {

	// lock the element
	eltLoan :=m.Lock(key)
	if eltLoan == nil {
		return false
	}


	m.mutex.Lock()
	defer m.mutex.Unlock()
	eltHolder := m.m[key]
	
	// remove from the map
	delete(m.m,eltHolder.element.Key())

	// close the waiting channel so that any callers waiting will fall-out
	close(eltHolder.lockQueue)

	return true
}