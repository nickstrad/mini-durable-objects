package store

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

// registry keeps each object's database open between requests. The first
// request activates the object (cold); later requests reuse it (warm). An
// object nobody has used for idleTimeout is closed and forgotten.
type registry struct {
	idleTimeout time.Duration

	mu      sync.Mutex
	objects map[string]*object
}

type object struct {
	db       *sql.DB
	active   int // requests currently using db
	lastUsed time.Time
}

func newRegistry(idleTimeout time.Duration) *registry {
	return &registry{idleTimeout: idleTimeout, objects: make(map[string]*object)}
}

// acquire returns id's database, opening path if the object is not active.
// Call release when done so the object can be evicted once idle.
func (r *registry) acquire(id, path string) (*sql.DB, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictIdle()

	obj, ok := r.objects[id]
	if !ok {
		db, err := openDB(path)
		if err != nil {
			return nil, nil, err
		}
		obj = &object{db: db}
		r.objects[id] = obj
		log.Printf("ACTIVATE %s", id)
	}
	obj.active++
	return obj.db, func() { r.release(obj) }, nil
}

func (r *registry) release(obj *object) {
	r.mu.Lock()
	defer r.mu.Unlock()
	obj.active--
	obj.lastUsed = time.Now()
}

// evictIdle closes objects that are not in use and have been idle too long.
// It is lazy: eviction only happens when a request comes in.
func (r *registry) evictIdle() {
	now := time.Now()
	for id, obj := range r.objects {
		if obj.active == 0 && now.Sub(obj.lastUsed) >= r.idleTimeout {
			obj.db.Close()
			delete(r.objects, id)
			log.Printf("EVICT %s", id)
		}
	}
}
