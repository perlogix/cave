package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	crypto    *Crypto
	sharedkey *AESKey
	metrics   map[string]interface{}
}

// KVUpdate type
type KVUpdate struct {
	UpdateType string   `json:"update_type"`
	Key        string   `json:"key"`
	Value      KVObject `json:"value"`
}

////////////////////////// IMPLEMENT ///////////////////////

//KVObject struct
type KVObject struct {
	LastUpdated time.Time       `json:"last_updated"`
	Secret      bool            `json:"secret"`
	Data        json.RawMessage `json:"data"`
	Locks       []Lock          `json:"locks"`
}

// Lock object
type Lock struct {
	Key         string    `json:"key"`
	Prefix      string    `json:"prefix"`
	LockID      string    `json:"lock_id"`
	NodeID      string    `json:"node_id"`
	NodeAddress string    `json:"node_address"`
	ClaimTime   time.Time `json:"claim_time"`
	ExpireTime  time.Time `json:"expire_time"`
}

////////////////////////////////////////////////////////////

func newKV(app *Bunker) (*KV, error) {
	kv := &KV{
		app:       app,
		terminate: make(chan bool),
		config:    app.Config,
		updates:   app.updates,
		log:       app.Logger,
		dbPath:    app.Config.KV.DBPath,
		crypto:    app.Crypto,
		metrics:   kvmetrics(),
	}
	start := time.Now()
	defer kv.doMetrics("startup", start)
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
	key, err := kv.crypto.UnsealSharedKey(kv.crypto.privkey)
	if err != nil {
		return kv, err
	}
	kv.sharedkey = key
	return kv, nil
}

func kvmetrics() map[string]interface{} {
	return map[string]interface{}{
		"pagefree": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_freelist_pages_free",
			Help: "Total number of free pages on the freelist",
		}),
		"pagepending": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_freelist_pages_pending",
			Help: "Total number of pending pages on the freelist",
		}),
		"pagealloc": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_freelist_bytes_total",
			Help: "Total bytes allocted in free pages",
		}),
		"pageuse": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_freelist_bytes_used",
			Help: "Total bytes used in the freelist",
		}),
		"tx_tot": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_tx_count",
			Help: "Total number of started read transactions",
		}),
		"tx_open": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_db_tx_open",
			Help: "Number of currently open read transactions",
		}),
		"transaction_time": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bunker_kv_transaction_time_ms",
			Help: "Duration of transactions by type",
		}, []string{"type"}),
		"dbsize": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_size_bytes",
			Help: "Size in bytes of the database on disk",
		}),
		"kv_q": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_kv_update_queue_size",
			Help: "Length of the KV update queue",
		}),
	}
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
func (kv *KV) Start() error {
	if kv.config.Mode == "dev" {
		return nil
	}
	err := kv.app.Cluster.network.RegisterService("kv", new(KVService))
	if err != nil {
		return err
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
	if kv.config.Mode == "dev" {
		return nil
	}
	start := time.Now()
	defer kv.doMetrics("emit:event", start)
	k := KVUpdate{
		UpdateType: t,
		Key:        key,
		Value:      value,
	}
	update, err := json.Marshal(k)
	if err != nil {
		return err
	}
	errs := kv.app.Cluster.network.CallAll("kv_update", nil, update)
	if len(errs) != 0 {
		kv.log.Error("Errors sending updates", errs)
		return fmt.Errorf("Errors sending updates")
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

func (kv *KV) doMetrics(tx string, start time.Time) {
	go func() {
		diff := time.Now().Sub(start)
		kv.metrics["transaction_time"].(*prometheus.GaugeVec).WithLabelValues(tx).Set(float64(diff.Milliseconds()))
		stats := kv.db.Stats()
		kv.metrics["pagefree"].(prometheus.Gauge).Set(float64(stats.FreePageN))
		kv.metrics["pagepending"].(prometheus.Gauge).Set(float64(stats.PendingPageN))
		kv.metrics["pagealloc"].(prometheus.Gauge).Set(float64(stats.FreeAlloc))
		kv.metrics["pageuse"].(prometheus.Gauge).Set(float64(stats.FreelistInuse))
		kv.metrics["tx_tot"].(prometheus.Gauge).Set(float64(stats.TxN))
		kv.metrics["tx_open"].(prometheus.Gauge).Set(float64(stats.OpenTxN))
		f, _ := os.Stat(kv.config.KV.DBPath)
		kv.metrics["dbsize"].(prometheus.Gauge).Set(float64(f.Size()))
	}()
}

func (kv *KV) getBuckets(tx *bbolt.Tx, buckets []string, prefix string, create bool) (*bbolt.Bucket, string, error) {
	start := time.Now()
	defer kv.doMetrics("get:buckets", start)
	var bkt *bbolt.Bucket
	var name string
	name = prefix
	bkt = tx.Bucket([]byte(prefix))
	if len(buckets) == 0 {
		return bkt, prefix, nil
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

//Put function
func (kv *KV) Put(key string, value []byte, prefix string, secret bool, e ...bool) error {
	return kv.PutObject(key, KVObject{
		LastUpdated: time.Now(),
		Secret:      secret,
		Data:        json.RawMessage(value),
		Locks:       []Lock{},
	}, prefix, secret, e...)
}

// PutObject value
func (kv *KV) PutObject(key string, value KVObject, prefix string, secret bool, e ...bool) error {
	start := time.Now()
	defer kv.doMetrics("put:key", start)
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
	buckets, k := parsePath(key)
	bobj, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, true)
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
		err = kv.emitEvent("put:key", key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get function
func (kv *KV) Get(key string, prefix string) ([]byte, error) {
	o, err := kv.GetObject(key, prefix)
	if err != nil {
		return nil, err
	}
	return o.Data, nil
}

// GetObject function
func (kv *KV) GetObject(key string, prefix string) (KVObject, error) {
	start := time.Now()
	defer kv.doMetrics("get:key", start)
	buckets, k := parsePath(key)
	bobj := []byte{}
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, false)
		if err != nil {
			return err
		}
		v := b.Get([]byte(k))
		bobj = v
		return nil
	})
	var obj KVObject
	err = json.Unmarshal(bobj, &obj)
	return obj, err

}

// GetKeys gets keys from a bucket
func (kv *KV) GetKeys(key string, prefix string) ([]string, error) {
	start := time.Now()
	defer kv.doMetrics("get:keys", start)
	buckets, k := parsePath(key)
	var keys []string
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, false)
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
func (kv *KV) DeleteKey(key string, prefix string, e ...bool) error {
	start := time.Now()
	defer kv.doMetrics("delete:key", start)
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
	buckets, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, false)
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
func (kv *KV) DeleteBucket(key string, prefix string, e ...bool) error {
	start := time.Now()
	defer kv.doMetrics("delete:bucket", start)
	emit := true
	if len(e) > 0 {
		emit = e[0]
	}
	buckets, k := parsePath(key)
	err := kv.db.Update(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, false)
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

// GetTree gets the db tree from the specified root to n-depth.
// If root is not given, it returns the entire db tree.
func (kv *KV) GetTree(prefix string, root ...string) (map[string]interface{}, error) {
	start := time.Now()
	defer kv.doMetrics("get:tree", start)
	startPath := ""
	if len(root) > 0 {
		startPath = root[0]
	}
	tree := map[string]interface{}{}
	buckets, k := parsePath(startPath)
	if k != "" {
		buckets = append(buckets, k)
	}
	fmt.Println(buckets, k)
	err := kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := kv.getBuckets(tx, buckets, prefix, false)
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

// Lock function
func (kv *KV) Lock(key string, prefix string, e ...bool) (Lock, error) {
	start := time.Now()
	defer kv.doMetrics("lock:create", start)
	id, err := machineid.ID()
	l := Lock{
		Key:         key,
		Prefix:      prefix,
		LockID:      uuid.New().String(),
		NodeID:      id,
		NodeAddress: kv.app.Cluster.node.Addr(),
		ClaimTime:   time.Now(),
		ExpireTime:  time.Now().Add(5 * time.Minute),
	}
	obj, err := kv.GetObject(key, prefix)
	if err != nil {
		return l, err
	}
	obj.Locks = append(obj.Locks, l)
	err = kv.PutObject(key, obj, prefix, obj.Secret, true)
	if err != nil {
		return l, err
	}
	return l, nil
}

// Unlock function
func (kv *KV) Unlock(lock Lock, e ...bool) error {
	start := time.Now()
	defer kv.doMetrics("lock:delete", start)
	obj, err := kv.GetObject(lock.Key, lock.Prefix)
	if err != nil {
		return err
	}
	index := -1
	for idx, l := range obj.Locks {
		if l.LockID == lock.LockID {
			index = idx
			break
		}
	}
	obj.Locks = append(obj.Locks[:index], obj.Locks[index+1:]...)
	err = kv.PutObject(lock.Key, obj, lock.Prefix, obj.Secret, true)
	if err != nil {
		return err
	}
	return nil
}

//KVService struct
type KVService struct {
	kv *KV
}

// PutKey function
func (k *KVService) PutKey(update KVUpdate) error {
	err := k.kv.PutObject(update.Key, update.Value, "kv", update.Value.Secret, false)
	if err != nil {
		return err
	}
	return nil
}

// DeleteKey func
func (k *KVService) DeleteKey(update KVUpdate) error {
	err := k.kv.DeleteKey(update.Key, "kv", false)
	if err != nil {
		return err
	}
	return nil
}

// DeleteBucket func
func (k *KVService) DeleteBucket(update KVUpdate) error {
	err := k.kv.DeleteBucket(update.Key, "kv", false)
	if err != nil {
		return err
	}
	return nil
}

// CreateLock func
func (k *KVService) CreateLock(update KVUpdate) error {
	_, err := k.kv.Lock(update.Key, "kv", false)
	if err != nil {
		return err
	}
	return nil
}

// DeleteLock func
func (k *KVService) DeleteLock(update KVUpdate) error {
	var l Lock
	err := json.Unmarshal(update.Value.Data, &l)
	if err != nil {
		return err
	}
	err = k.kv.Unlock(l, false)
	if err != nil {
		return err
	}
	return nil
}

// SyncAll function
func (k *KVService) SyncAll() (map[string]json.RawMessage, error) {
	db := map[string]json.RawMessage{}
	err := k.kv.db.View(func(tx *bbolt.Tx) error {
		b, _, err := k.kv.getBuckets(tx, []string{}, "kv", false)
		if err != nil {
			return err
		}
		db = listBucket(b, "/")
		return nil
	})
	if err != nil {
		return nil, err
	}
	k.kv.log.Pretty(db)
	return db, nil
}

func listBucket(bkt *bbolt.Bucket, path string) map[string]json.RawMessage {
	c := bkt.Cursor()
	db := map[string]json.RawMessage{}
	for ea, v := c.First(); ea != nil; ea, v = c.Next() {
		isBucket := bkt.Bucket(ea)
		if isBucket != nil {
			t := listBucket(isBucket, path+string(ea[:])+"/")
			for k, v := range t {
				db[k] = v
			}
		} else {
			db[path+string(ea[:])] = json.RawMessage(v)
		}

	}
	return db
}
