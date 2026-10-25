package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryReusesWarmObject(t *testing.T) {
	r := newRegistry(time.Hour)
	path := filepath.Join(t.TempDir(), "a.sqlite")

	first, release, err := r.acquire("a", path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	second, release, err := r.acquire("a", path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if first != second {
		t.Fatal("warm object was reopened")
	}
}

func TestRegistryEvictsOnlyIdleObjects(t *testing.T) {
	r := newRegistry(0) // Every released object is immediately idle.
	dir := t.TempDir()

	_, releaseA, err := r.acquire("a", filepath.Join(dir, "a.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, releaseB, err := r.acquire("b", filepath.Join(dir, "b.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	releaseB()
	if _, ok := r.objects["a"]; !ok {
		t.Fatal("object in use was evicted")
	}

	releaseA()
	_, releaseB, err = r.acquire("b", filepath.Join(dir, "b.sqlite"))
	if err != nil {
		t.Fatal(err)
	}

	releaseB()
	if _, ok := r.objects["a"]; ok {
		t.Fatal("idle object was not evicted")
	}
}
