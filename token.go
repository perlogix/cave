package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TokenStore manages cluster tokens
type TokenStore struct {
	app       *Cave
	terminate chan bool
	config    *Config
	tokens    chan Message
	log       *Log
	metrics   map[string]interface{}
	store     map[string]Token
	lock      sync.Mutex
}

// NewTokenStore function
func NewTokenStore(app *Cave) (*TokenStore, error) {
	t := &TokenStore{
		app:       app,
		config:    app.Config,
		log:       app.Logger,
		terminate: make(chan bool),
		tokens:    app.tokens,
		metrics:   tokenMetrics(),
		lock:      sync.Mutex{},
		store:     map[string]Token{},
	}
	return t, nil
}

func tokenMetrics() map[string]interface{} {
	return map[string]interface{}{
		"token_count": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_tokens_token_count",
			Help: "The number of tokens in the token store",
		}),
		"token_detail": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_tokens_token_details",
			Help: "Tokens broken down by details",
		}, []string{"issue_time", "expire_time", "user", "issued_by", "type", "token_redacted"}),
		"tokens_issued": promauto.NewCounter(prometheus.CounterOpts{
			Name: "cave_tokens_issued_count",
			Help: "Number of issued tokens",
		}),
		"tokens_issued_detail": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_tokens_issue_detail",
			Help: "Tokens issued, detail view",
		}, []string{"issue_time", "expire_time", "user", "issued_by", "type", "token_redacted"}),
		"tokens_deleted": promauto.NewCounter(prometheus.CounterOpts{
			Name: "cave_tokens_deleted_count",
			Help: "Number of tokens deleted",
		}),
		"token_cleanup": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_tokens_cleanup_cycle_duration",
			Help: "Time taken for cleaning up tokens",
		}),
		"token_cleanup_count": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_tokens_cleanup_cycle_count",
			Help: "Number of tokens cleaned during cycle",
		}),
		"token_cleanup_run_count": promauto.NewCounter(prometheus.CounterOpts{
			Name: "cave_tokens_cleanup_cycle_run_count",
			Help: "Number of times the cleanup cycle has run",
		}),
		"timings": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_tokens_transaction_times_ms",
			Help: "Transaction times for token functions in ms",
		}, []string{"function"}),
		"update_queue": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_tokens_update_queue_len",
			Help: "Length of the update queue",
		}),
	}
}

// Start function
func (t *TokenStore) Start() {
	idx := 0
	for {
		idx++
		go func() {
			t.metrics["update_queue"].(prometheus.Gauge).Set(float64(len(t.tokens)))
			t.metrics["token_count"].(prometheus.Gauge).Set(float64(len(t.store)))
			for _, v := range t.store {
				t.metrics["token_detail"].(*prometheus.GaugeVec).WithLabelValues(v.IssueTime.String(), v.ExpireTime.String(), v.UID, v.IssuedBy, v.Type, fmt.Sprintf("xxxx-%s", v.Token[24:])).Set(1.0)
			}
		}()
		select {
		case <-t.terminate:
			return
		case m := <-t.tokens:
			err := t.HandleUpdate(m)
			if err != nil {
				t.log.Error(err)
			}
		default:
			// clean up tokens every 100 runs or so
			if idx%100 == 0 {
				start := time.Now()
				idx = 0
				del := []string{}
				for k, v := range t.store {
					if time.Now().Sub(v.IssueTime) <= 0 || time.Now().Sub(v.ExpireTime) > 0 {
						del = append(del, k)
					}
				}
				for _, i := range del {
					err := t.Delete(i)
					if err != nil {
						t.log.Error(err)
					}
				}
				go func() {
					t.metrics["token_cleanup"].(prometheus.Gauge).Set(float64(time.Now().Sub(start).Milliseconds()))
					t.metrics["token_cleanup_count"].(prometheus.Gauge).Set(float64(len(del)))
					t.metrics["token_cleanup_run_count"].(prometheus.Counter).Inc()

				}()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// HandleUpdate func
func (t *TokenStore) HandleUpdate(m Message) error {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("handle_update").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	var d map[string]Token
	err := json.Unmarshal(m.Data, &d)
	if err != nil {
		return err
	}
	t.lock.Lock()
	t.store = d
	t.lock.Unlock()
	return nil
}

// Find function
func (t *TokenStore) Find(tid string) (Token, error) {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("find").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	if val, ok := t.store[tid]; ok {
		return val, nil
	}
	return Token{}, fmt.Errorf("Token does not exist")
}

// Validate function
func (t *TokenStore) Validate(typ string, uid string, tid string) bool {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("validate").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	token, err := t.Find(tid)
	if err != nil {
		return false
	}
	if time.Now().Sub(token.IssueTime) >= 0 && time.Now().Sub(token.ExpireTime) < 0 && uid == token.UID && typ == token.Type {
		return true
	}
	return false
}

// FindUser function
func (t *TokenStore) FindUser(uid string) (Token, error) {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("find_user").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	for _, v := range t.store {
		if v.UID == uid {
			return v, nil
		}
	}
	return Token{}, fmt.Errorf("User does not have any issued tokens")
}

// Delete function
func (t *TokenStore) Delete(tid string) error {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("delete").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	if _, ok := t.store[tid]; ok {
		t.lock.Lock()
		delete(t.store, tid)
		t.lock.Unlock()
		return nil
	}
	err := t.emit()
	if err != nil {
		return err
	}
	go t.metrics["tokens_deleted"].(prometheus.Counter).Inc()
	return fmt.Errorf("Token does not exist")
}

// Issue function
func (t *TokenStore) Issue(user string, typ string, expire ...bool) (Token, error) {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("issue").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	e := time.Now().Add(60 * time.Minute)
	if len(expire) > 0 && expire[0] == false {
		e = time.Now().Add(87600 * time.Hour)
	}
	token := uuid.New().String()
	tok := Token{
		IssueTime:  time.Now(),
		ExpireTime: e,
		UID:        user,
		Type:       typ,
		IssuedBy:   t.app.Crypto.id,
		Token:      token,
	}
	t.lock.Lock()
	t.store[token] = tok
	t.lock.Unlock()
	err := t.emit()
	if err != nil {
		return tok, err
	}
	go func() {
		t.metrics["tokens_issued"].(prometheus.Counter).Inc()
		t.metrics["tokens_issued_detail"].(*prometheus.GaugeVec).WithLabelValues(tok.IssueTime.String(), tok.ExpireTime.String(), tok.UID, tok.IssuedBy, tok.Type, fmt.Sprintf("xxxx-%s", tok.Token[24:])).Set(1.0)
	}()
	return tok, nil
}

func (t *TokenStore) emit() error {
	start := time.Now()
	defer func() {
		t.metrics["timings"].(*prometheus.GaugeVec).WithLabelValues("emit").Set(float64(time.Now().Sub(start).Milliseconds()))
	}()
	if t.config.Mode == "dev" {
		return nil
	}
	t.lock.Lock()
	b, err := json.Marshal(t.store)
	if err != nil {
		return err
	}
	t.lock.Unlock()
	err = t.app.Cluster.Emit("token", b, "token:update")
	if err != nil {
		return err
	}
	return nil
}
