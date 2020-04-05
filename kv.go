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
	UpdateType string   `json:"update_type"`
	Key        string   `json:"key"`
	Value      KVObject `json:"value"`
}

//KVObject type defines the data type thats actually saved to the kv store
type KVObject struct {
	Data       []byte    `json:"data"`
	DataType   string    `json:"data_type"`
	LastChange time.Time `json:"last_change"`
}

//KVStats struct
type KVStats struct {
}

func newKV(app *Bunker) (*KV, error) {
	kv := &KV{
		app:       app,
		terminate: make(chan bool),
		config:    app.Config,
		updates:   app.updates,
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
				kv.log.Error(err)
			}
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (kv *KV) handleUpdate(msg Message) error {
	kv.log.Debug("GOT EVENT")
	var kvu KVUpdate
	err := json.Unmarshal(msg.Data, &kvu)
	if err != nil {
		return err
	}
	switch kvu.UpdateType {
	case "put:key":
		err := kv.Put(kvu.Key, kvu.Value.Data, kvu.Value.DataType, false)
		if err != nil {
			return err
		}
	case "delete:key":
		err := kv.DeleteKey(kvu.Key, false)
		if err != nil {
			return err
		}
	case "delete:bucket":
		err := kv.DeleteBucket(kvu.Key, false)
		if err != nil {
			return err
		}
	default:
		kv.log.Error("UpdateType " + kvu.UpdateType + " not a valid type")
		return nil
	}
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

func (kv *KV) emitEvent(t string, key string, value KVObject) error {
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
	var bkt *bbolt.Bucket
	var name string
	name = "kv"
	bkt = tx.Bucket([]byte("kv"))
	if len(buckets) == 0 {
		return bkt, "kv", nil
	}
	for _, b := range buckets {
		var err error
		if create {
			bkt, err = bkt.CreateBucketIfNotExists([]byte(b))
			if err != nil {
				return bkt, name, err
			}
		} else {
			bkt = bkt.Bucket([]byte(b))
			if bkt == nil {
				return nil, b, fmt.Errorf("Bucket " + b + " does not exist")
			}
		}
		name = b
	}
	return bkt, name, nil
}

// Put value
func (kv *KV) Put(key string, value []byte, dataType string, e ...bool) error {
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
	buckets, k := parsePath(key)
	obj := KVObject{
		Data:       value,
		DataType:   dataType,
		LastChange: time.Now(),
	}
	bobj, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	err = kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, true)
		if err != nil {
			return err
		}
		err = b.Put([]byte(k), bobj)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if emit {
		err = kv.emitEvent("put:key", key, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get function
func (kv *KV) Get(key string) (KVObject, error) {
	buckets, k := parsePath(key)
	var obj KVObject
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		v := b.Get([]byte(k))
		err = json.Unmarshal(v, &obj)
		if err != nil {
			return err
		}
		return nil
	})
	return obj, err
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
func (kv *KV) DeleteKey(key string, e ...bool) error {
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
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
	if emit {
		err = kv.emitEvent("delete:key", key, KVObject{})
		if err != nil {
			return err
		}
	}
	return err
}

// DeleteBucket function
func (kv *KV) DeleteBucket(key string, e ...bool) error {
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
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
	if emit {
		err = kv.emitEvent("delete:bucket", key, KVObject{})
		if err != nil {
			return err
		}
	}
	return err
}
