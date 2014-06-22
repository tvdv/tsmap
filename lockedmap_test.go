package tsmap

import (
"testing"
"runtime"
"sync"
"time"
)

type TestElement struct {
	a int
	b string
	key string
}

func (e *TestElement) Key() string {
	return e.key
}

// Test doing an add
func TestLockedMapAdd (t *testing.T) {
	m := CreateLockedMap()

	e1 := &TestElement {
		a: 5,
		b: "hello",
		key :"key1",
	}
	if m.Add(e1) != true {
		t.Error("Add did not return true")
	}

	//second add should fail, due to same key
	if m.Add(e1) != false {
		t.Error("Add did not return false")
	}

	//make sure a different key works
	e2 := &TestElement {
		a: 5,
		b: "hello",
		key :"key2",
	}
	if m.Add(e2) == false {
		t.Error("Add should have passed")
	}


	// try deleting an element

	ret := m.Delete(e2.Key())
	if ret != true {
		t.Error("Delete should have passed")
	}


	// Test deleting an element whilst others are trying to lock.
	// Test and (expected behaviour)
	// 1. A locks element (and gets it)
	// 2. B deletes element (blocks)
	// 3. C locks element (blocks)
	// 3. A unlocks element (allowing B to delete)
	// 4. (C's lock request returns as a failure)

	// A lock an element
	eltLoan := m.Lock(e1.key)
	if eltLoan == nil {
		t.Error("Lock failed")
	}

	
	deleteEndChan := make(chan int)
	ClockResultChan := make(chan bool)
	

	CLockFn := func (lockResult chan bool) {
		ret := m.Lock(e1.key)
		if ret == nil {
			lockResult <- true
		} else {
			lockResult <- false
		}
	}

	BDeleteFn := func(doneChan chan int) {
		ret = m.Delete(e1.key)
		if ret != true {
			t.Error("Delete should have passed")
		}
		doneChan <- 0
	}


	go BDeleteFn(deleteEndChan)
	// Wait for B to get blocked on the channel
	time.Sleep(time.Millisecond*300) // I swear officer, I was just going to fix this!

	go CLockFn(ClockResultChan)
	// Wait for C to call Lock
	time.Sleep(time.Millisecond*300) // I swear officer, I was just going to fix this!

	// get A to unlock
	m.Unlock(eltLoan)

	//wait for delete to finish
	<- deleteEndChan

	//check that C returned with the correct
	delayChan := time.After(time.Millisecond*500)
	select {
	case cret := <- ClockResultChan:
		if cret != true {
			t.Error("C's lock did not fail as expected")
		}
	case <-delayChan:
		t.Error("C's Lock fn never returned")

	}
	
}


// Test to make sure that a read-modify-write cycle (with Lock and Unlock) works
func TestLockedMapEdit (t *testing.T) {
	m := CreateLockedMap()
	src_e1 := &TestElement {
		a: 5,
		b: "hello",
		key :"key1",
	}
	if m.Add(src_e1) != true {
		t.Error("Add did not return true")
	}

	eltLoan :=m.Lock(src_e1.key)

	if eltLoan == nil {
		t.Error("couldn't get item")
	}

	e1 := eltLoan.Element().(*TestElement)
	e1.a=e1.a +1
	e1.b="new"

	if m.Unlock(eltLoan) !=true {
		t.Error("couldn't return item")
	}

	//make sure the loan is no longer valid
	if eltLoan.Element() != nil {
		t.Error("loan shouldn't still hand out element")
	}

	//Retrieve again and make sure changes were saved
	eltLoan2 :=m.Lock(e1.key)


	if eltLoan2 == nil {
		t.Error("couldn't get item second time")
	}

	e2 := eltLoan2.Element().(*TestElement)

	if e2.a != 6 {
		t.Error("wrong values")
	}
	if e2.b != "new" {
		t.Error("wrong values")
	}
}

// Run a lot of go routines that increment a counter inside a map element
// if the count at the end isn't equal to the number of go routines then
// there is a thread safety issue
func TestThreadSafety(t *testing.T) {
	if runtime.GOMAXPROCS(-1) == 1 {
		//have more than 1 cpu to really test thread safety
		runtime.GOMAXPROCS(4)
	}

	m := CreateLockedMap()
	src_e1 := &TestElement {
		a: 0,
		b: "hello",
		key :"key1",
	}
	if m.Add(src_e1) != true {
		t.Error("Add did not return true")
	}

	workerFunc := func (m *LockedMap, wg *sync.WaitGroup) {
		eltLoan := m.Lock("key1")
		if (eltLoan == nil) {
			t.Error("Lock failed")
			return
		}
		eltInterface := eltLoan.Element()
		if eltInterface == nil {
			t.Error("Element was nil")
		} else {
			e1 := eltInterface.(*TestElement)
			e1.a = e1.a + 1
		}
		
		if !m.Unlock(eltLoan) {
			t.Error("unlock failed")
			panic("thread block")
		}

		wg.Done()
	}

	total := 10000
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 1; i <= total; i++ {
		go workerFunc(m,&wg)
	}

	wg.Wait()

	eltLoan := m.Lock("key1")
	if (eltLoan == nil) {
		t.Error("Lock failed")
	}
	e1 := eltLoan.Element().(*TestElement)
	if e1.a != total {
		t.Error("count does not match")
	}
}

/*
// Test to confirm behaviour of the sync.Mutex if you call Unlock twice
func TestMutexDoubleUnlock(t *testing.T) {
	mutex := sync.Mutex{}
	mutex.Lock()
	mutex.Unlock()
	mutex.Unlock() // panic occurs here (when tested on go version go1.1.2 linux/amd64)

	//check for normal op
	mutex.Lock()
	mutex.Unlock()
}
*/
