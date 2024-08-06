package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"testing"
)

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}

func TestPathTransformfunc(t *testing.T) {
	key := "somethingToTest"

	pathKey := CASPathTransformFunc(key)
	log.Println("Pathname", pathKey.Filename)
}

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}

	store := NewStore(opts)

	defer teardown(t, store)

	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("TestingKey_%d", i)
		data := bytes.NewReader([]byte("For testing"))

		_, err := store.Write(key, data)
		if err != nil {
			log.Printf("Error writing to store %s \n", err)
			continue
		}

		_, readData, err := store.Read(key)

		if err != nil {
			log.Printf("Error reading data %s \n", err)
			continue
		}
		b, err := io.ReadAll(readData)

		if err != nil {
			t.Error(err)
		}

		log.Printf("Data: %s \n", string(b))
	}
}
