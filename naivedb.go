package naivedb

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type ReaderStringWriter interface {
	io.Reader
	io.StringWriter
	io.Seeker
}

type NaiveDB struct {
	store     ReaderStringWriter
	hintStore io.ReadWriter
	offsetMap map[string]int64
}

type FileBackedNaiveDB struct {
	db   NaiveDB
	file *os.File
}

func (db *NaiveDB) attemptLoadOffsetMap() {
	dec := gob.NewDecoder(db.hintStore)
	if err := dec.Decode(&db.offsetMap); err != nil {
		log.Fatalln(err)
	}

	log.Printf("loaded offset map %v", db.offsetMap)
}

func (db *NaiveDB) attemptSaveOffsetMap() {
	enc := gob.NewEncoder(db.hintStore)
	if err := enc.Encode(db.offsetMap); err != nil {
		log.Fatalln(err)
	}

	log.Printf("saved offset map %v", db.offsetMap)
}

// func NewNaiveDB(store ReaderStringWriter, hintStore io.ReadWriter) (NaiveDB, error) {
// 	offsetMap := make(map[string]int64)
// 	db := NaiveDB{store, hintStore, offsetMap}
// 	return db, nil
// }

func NewFileBackedNaiveDB(filename string) (_ *FileBackedNaiveDB, err error) {
	store, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	var db NaiveDB

	hintStoreFilename := fmt.Sprintf("%s.hint", filename)
	hintStore, err := os.OpenFile(hintStoreFilename, os.O_RDWR, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			hintStore, err = os.Create(hintStoreFilename)
			if err != nil {
				log.Fatalln(err)
			}
			offsetMap := make(map[string]int64)
			db = NaiveDB{store, hintStore, offsetMap}
			err = nil
		} else {
			log.Fatalln(err)
		}
	} else {
		offsetMap := make(map[string]int64)
		db = NaiveDB{store, hintStore, offsetMap}
		db.attemptLoadOffsetMap()
	}

	return &FileBackedNaiveDB{db, store}, err
}

func (db *FileBackedNaiveDB) Set(key string, value string) (err error) {
	err = db.db.set(key, value)
	if err != nil {
		db.file.Close() // ignore error; Write error takes precedence
		return err
	}

	return nil
}

func (db *FileBackedNaiveDB) Get(key string) (value string, err error) {
	return db.db.get(key)
}

func (db *NaiveDB) set(key string, value string) (err error) {
	currentOffset, err := db.store.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// fmt.Println(currentOffset)

	_, err = db.store.WriteString(fmt.Sprintf("%s,%s\n", key, value))
	db.offsetMap[key] = currentOffset

	db.attemptSaveOffsetMap()

	return err
}

func (db *NaiveDB) get(key string) (value string, err error) {
	offset, ok := db.offsetMap[key]
	if !ok {
		panic("oh no")
	}

	db.store.Seek(offset, io.SeekStart)

	scanner := bufio.NewScanner(db.store)
	for scanner.Scan() {
		_, err := db.store.Seek(0, io.SeekCurrent)
		if err != nil {
			log.Fatalln(err)
		}

		line := scanner.Text()

		if strings.Contains(line, key) {
			value = strings.Split(line, ",")[1]
		}
	}

	err = scanner.Err()
	return value, err
}
