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
		terminate: make(chan bool),
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
	kv.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("kv"))
		if err != nil {
			return err
		}
		return nil
	})
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
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (kv *KV) handleUpdate(msg Message) error {
	kv.log.Pretty(msg)
	return nil
}

func (kv *KV) handleSync(msg Message) error {
	kv.log.Pretty(msg)
	return nil
}

func (kv *KV) handleEvent(msg Message) error {
	kv.log.Pretty(msg)
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
	err = kv.app.Cluster.Emit("update", update, "KVUpdate")
	if err != nil {
		return err
	}
	return nil
}

func parsePath(path string) (buckets []string, key string) {
	last := path[strings.LastIndex(path, "/")+1:]
	if last == path {
		return []string{}, last
	}
	// return path without key name, key name
	paths := strings.Split(path, "/")
	return paths[:len(paths)-1], last
}

func (kv *KV) getBuckets(tx *bbolt.Tx, buckets []string, create bool) (*bbolt.Bucket, string, error) {
	if len(buckets) == 0 {
		return nil, "", nil
	}
	var bkt *bbolt.Bucket
	var name string
	for _, b := range buckets {
		var err error
		if create {
			bkt, err = tx.CreateBucketIfNotExists([]byte(b))
			if err != nil {
				return bkt, name, err
			}
		} else {
			bkt = tx.Bucket([]byte(b))
		}
		name = b
	}
	return bkt, name, nil
}

// Put value
func (kv *KV) Put(key string, value []byte) error {
	buckets, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, true)
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
	err = kv.emitEvent("put:key", key, value)
	if err != nil {
		return err
	}
	return nil
}

// Get function
func (kv *KV) Get(key string) ([]byte, error) {
	buckets, k := parsePath(key)
	kv.log.Info(buckets, k)
	var val []byte
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		v := b.Get([]byte(k))
		val = v
		return nil
	})
	return val, err
}

// GetKeys gets keys from a bucket
func (kv *KV) GetKeys(key string) ([]string, error) {
	buckets, k := parsePath(key)
	var keys []string
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		var bkt *bbolt.Bucket
		if k == "" {
			bkt = b
		} else {
			bkt = b.Bucket([]byte(k))
		}
		c := bkt.Cursor()
		txKeys := []string{}
		for ea, _ := c.First(); ea != nil; ea, _ = c.Next() {
			txKeys = append(txKeys, string(ea[:]))
		}
		keys = txKeys
		return nil
	})
	return keys, err
}

// DeleteKey function
func (kv *KV) DeleteKey(key string) error {
	buckets, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		err = b.Delete([]byte(k))
		if err != nil {
			return err
		}
		return nil
	})
	err = kv.emitEvent("delete:key", key, []byte{})
	if err != nil {
		return err
	}
	return err
}

// DeleteBucket function
func (kv *KV) DeleteBucket(key string) error {
	buckets, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		err = b.DeleteBucket([]byte(k))
		if err != nil {
			return err
		}
		return nil
	})
	err = kv.emitEvent("delete:bucket", key, []byte{})
	if err != nil {
		return err
	}
	return err
}
