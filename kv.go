package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// KV type
type KV struct {
	app       *Bunker
	terminate chan bool
	config    *Config
	events    chan Message
	updates   chan Message
	sync      chan Message
	db        *bbolt.DB
	dbPath    string
	log       *Log
}

// KVUpdate type
type KVUpdate struct {
	UpdateType string `json:"update_type"`
	Key        string `json:"key"`
	Value      []byte `json:"value"`
}

func newKV(app *Bunker) (*KV, error) {
	kv := &KV{
		app:       app,
		terminate: make(chan bool, 1),
		config:    app.Config,
		events:    app.events,
		updates:   app.updates,
		sync:      app.sync,
		log:       app.Logger,
		dbPath:    app.Config.KV.DBPath,
	}
	if _, err := os.Stat(kv.dbPath); os.IsNotExist(err) {
		p := strings.Split(kv.dbPath, "/")
		if len(p) > 1 {
			s := p[:len(p)-1]
			q := strings.Join(s, "/")
			err := os.MkdirAll(q, 0755)
			if err != nil {
				return kv, err
			}
		}
	}
	options := &bbolt.Options{
		Timeout:      30 * time.Second,
		FreelistType: "hashmap",
	}
	db, err := bbolt.Open(kv.dbPath, 0755, options)
	if err != nil {
		return kv, err
	}
	kv.db = db
	return kv, nil
}

// Start func
func (kv *KV) Start() {
	for {
		select {
		case <-kv.terminate:
			return
		case msg := <-kv.updates:
			err := kv.handleUpdate(msg)
			if err != nil {
				fmt.Println(err)
			}

		case msg := <-kv.events:
			err := kv.handleEvent(msg)
			if err != nil {
				fmt.Println(err)
			}
		case msg := <-kv.sync:
			err := kv.handleSync(msg)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func (kv *KV) handleUpdate(msg Message) error {
	fmt.Println(msg)
	return nil
}

func (kv *KV) handleSync(msg Message) error {
	fmt.Println(msg)
	return nil
}

func (kv *KV) handleEvent(msg Message) error {
	fmt.Println(msg)
	return nil
}

func (kv *KV) emitEvent(t string, key string, value []byte) error {
	k := KVUpdate{
		UpdateType: t,
		Key:        key,
		Value:      value,
	}
	update, err := json.Marshal(k)
	if err != nil {
		return err
	}
	err = kv.app.Cluster.Emit("kv_update", update, "KVUpdate")
	if err != nil {
		return err
	}
	return nil
}

func parsePath(path string) (bucket string, key string) {
	last := path[strings.LastIndex(path, "/")+1:]
	if last == path {
		return "", last
	}
	// return path without key name, key name
	return strings.Join(strings.Split(path, "/")[:len(path)-1], "/"), last
}

// Put value
func (kv *KV) Put(key string, value []byte) error {
	bucket, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		err = b.Put([]byte(k), value)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = kv.emitEvent("put", key, value)
	if err != nil {
		return err
	}
	return nil
}

// Get function
func (kv *KV) Get(key string) ([]byte, error) {
	bucket, k := parsePath(key)
	var val []byte
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		val = b.Get([]byte(k))
		return nil
	})
	if err != nil {
		return []byte{}, err
	}
	return val, nil
}
