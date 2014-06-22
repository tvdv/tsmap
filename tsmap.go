// provides a thread-safe map. Allows for thread-safe read-modify-writes to elements.
//
// Currently there is only one implementation of the thread-safe map, LockedMap.
// LockedMap uses sync primitivtes to ensure thread safety.
//
// The following assumptions were made when developing tsmap:
// 1. Each element is a relatively large structure (rather than say a single int),
//   so the overhead of storing elements as interfaces shouldn't be high
// 2. Each element reports its key via the Key() method on the Element interface.
//   This method must always return the same key, otherwise behaviour is undefined.
package tsmap

// Interface that all elements added to the Map must conform to
type Element interface {
	// return the key of element. This function must always return the same
	// key for a given element.
	Key() string
}

// Returned by Map when locking an element
// must be returned to Unlock
type MapElementAccess interface {
	Element() Element
}

type Map interface {
	// Add element to the map
	//
	// If an element already exists at that key, false is returned
	// otherwise the element is added and true returned. The caller MUST obtain
	// a lock to perform any further manipulations the element.
	Add(element Element) bool

	// blocks until the element is obtained. If the element doesn't exist, return value is nil
	// The caller must return the element with Unlock, when finished.
	Lock(key string) (MapElementAccess)

	// return a locked element.
	// returns false if the element has already been returned
	Unlock(loan MapElementAccess) bool

	Delete(key string) bool
}