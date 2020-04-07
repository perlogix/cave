package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	options   *bbolt.Options
}

// KVUpdate type
type KVUpdate struct {
	UpdateType string `json:"update_type"`
	Key        string `json:"key"`
	Value      []byte `json:"value"`
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
	kv.options = &bbolt.Options{
		Timeout:      30 * time.Second,
		FreelistType: "hashmap",
	}
	db, err := dbOpen(kv.dbPath, kv.options)
	if err != nil {
		return kv, err
	}
	kv.db = db
	kv.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("kv"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("_system"))
		if err != nil {
			return err
		}
		return nil
	})
	return kv, nil
}

func dbOpen(path string, options *bbolt.Options) (*bbolt.DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		p := strings.Split(path, "/")
		if len(p) > 1 {
			s := p[:len(p)-1]
			q := strings.Join(s, "/")
			err := os.MkdirAll(q, 0755)
			if err != nil {
				return &bbolt.DB{}, err
			}
		}
	}
	db, err := bbolt.Open(path, 0755, options)
	if err != nil {
		return db, err
	}
	return db, nil
}

func dbClose(db *bbolt.DB) error {
	err := db.Close()
	if err != nil {
		return err
	}
	return nil
}

// Start func
func (kv *KV) Start() {
	if kv.app.rsa == nil {
		err := kv.loadRSA()
		if err != nil {
			panic(err)
		}
	}
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

func (kv *KV) writeRSA(pem []byte) error {
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte("_system"))
		if bkt == nil {
			return fmt.Errorf("_system bucket does not exist")
		}
		err := bkt.Put([]byte("rsa"), pem)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func (kv *KV) loadRSA() error {
	var b []byte
	err := kv.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte("_system"))
		if bkt == nil {
			return fmt.Errorf("_system bucket does not exist")
		}
		v := bkt.Get([]byte("rsa"))
		if len(v) == 0 {
			return fmt.Errorf("RSA key is blank")
		}
		b = v
		return nil
	})
	if err != nil {
		return err
	}
	rawPem, _ := pem.Decode(b)
	privKey, err := x509.ParsePKCS1PrivateKey(rawPem.Bytes)
	if err != nil {
		return err
	}
	kv.app.rsa = privKey
	return nil
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
		err := kv.Put(kvu.Key, kvu.Value, false)
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
func (kv *KV) Put(key string, value []byte, e ...bool) error {
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
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
	if emit {
		err = kv.emitEvent("put:key", key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get function
func (kv *KV) Get(key string) ([]byte, error) {
	buckets, k := parsePath(key)
	obj := []byte{}
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		v := b.Get([]byte(k))
		obj = v
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
			isBucket := bkt.Bucket(ea)
			dir := ""
			if isBucket != nil {
				dir = "/"
			}
			txKeys = append(txKeys, string(ea[:])+dir)
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
		err = kv.emitEvent("delete:key", key, []byte{})
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
		err = kv.emitEvent("delete:bucket", key, []byte{})
		if err != nil {
			return err
		}
	}
	return err
}

// GetTree gets the db tree from the specified root to n-depth.
// If root is not given, it returns the entire db tree.
func (kv *KV) GetTree(root ...string) (map[string]interface{}, error) {
	startPath := ""
	if len(root) > 0 {
		startPath = root[0]
	}
	tree := map[string]interface{}{}
	buckets, k := parsePath(startPath)
	if k != "" {
		buckets = append(buckets, k)
	}
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, false)
		if err != nil {
			return err
		}
		tree = enumerateBucket(b)
		return nil
	})
	if err != nil {
		return tree, err
	}
	return tree, nil
}

func enumerateBucket(bkt *bbolt.Bucket) map[string]interface{} {
	c := bkt.Cursor()
	tree := map[string]interface{}{}
	for ea, v := c.First(); ea != nil; ea, v = c.Next() {
		isBucket := bkt.Bucket(ea)
		if isBucket != nil {
			tree[string(ea[:])] = enumerateBucket(isBucket)
		} else {
			tree[string(ea[:])] = json.RawMessage(v)
		}
	}
	return tree
}
