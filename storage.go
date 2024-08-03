package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const DefaultRootFolder = "mrbunkar"

func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))

	hashStr := hex.EncodeToString(hash[:])
	blockSize := 10
	sliceSize := len(hashStr) / blockSize

	path := make([]string, sliceSize)
	for i := 0; i < sliceSize; i++ {
		from, to := i*blockSize, i*blockSize+blockSize
		path[i] = hashStr[from:to]
	}

	return PathKey{
		PathName: strings.Join(path, "/"),
		Filename: hashStr,
	}

}

type PathKey struct {
	PathName string
	Filename string
}

func (k PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", k.PathName, k.Filename)
}

func (k PathKey) FirstPathName() string {
	path := strings.Split(k.PathName, "/")

	if len(path) == 0 {
		return ""
	}

	return path[0]
}

type PathTransformFunc func(string) PathKey

type StoreOpts struct {
	Root              string
	PathTransformFunc PathTransformFunc
}

type Store struct {
	Opts StoreOpts
}

func NewStore(opts StoreOpts) *Store {

	if len(opts.Root) == 0 {
		opts.Root = DefaultRootFolder
	}

	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = CASPathTransformFunc
	}

	return &Store{
		Opts: opts,
	}
}

func (s *Store) Has(key string) bool {
	pathKey := s.Opts.PathTransformFunc(key)

	_, err := os.Stat(pathKey.FullPath())
	if err != nil {
		return false
	}

	return true
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Opts.Root)
}

func (s *Store) HasKey(key string) bool {
	pathKey := s.Opts.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s", s.Opts.Root, pathKey.FullPath())

	if _, err := os.Stat(fullPathWithRoot); errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

func (s *Store) Delete(key string) error {
	pathKey := s.Opts.PathTransformFunc(key)

	defer func() {
		log.Printf("Delete [%s] from disk", pathKey.Filename)
	}()

	if s.HasKey(key) == false {
		return nil
	}

	firstPathNameWithRoot := fmt.Sprintf("%s/%s", s.Opts.Root, pathKey.FirstPathName())

	return os.RemoveAll(firstPathNameWithRoot)
}

func (s *Store) Read(key string) (int, io.Reader, error) {
	return s.readStream(key)
}

func (s *Store) readStream(key string) (int, io.ReadCloser, error) {
	pathKey := s.Opts.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s", s.Opts.Root, pathKey.FullPath())

	file, err := os.Open(fullPathWithRoot)

	if err != nil {
		return 0, nil, err
	}

	fi, err := file.Stat()

	if err != nil {
		return 0, nil, err
	}

	return int(fi.Size()), file, nil
}

func (s *Store) Write(key string, r io.Reader) (int64, error) {
	return s.writeStream(key, r)
}

func (s *Store) writeStream(key string, r io.Reader) (int64, error) {
	pathKey := s.Opts.PathTransformFunc(key)

	pathNameWithRoot := fmt.Sprintf("%s/%s", s.Opts.Root, pathKey.PathName)

	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return 0, err
	}

	fullPathWithRoot := fmt.Sprintf("%s/%s", s.Opts.Root, pathKey.FullPath())

	f, err := os.Create(fullPathWithRoot)
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}
	fmt.Printf("[%d] bytes written to store \n", n)
	return n, nil
}
