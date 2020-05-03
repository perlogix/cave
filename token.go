package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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
	}
	return t, nil
}

func tokenMetrics() map[string]interface{} {
	return map[string]interface{}{}
}

// Start function
func (t *TokenStore) Start() {
	for {
		select {
		case m := <-t.tokens:
			err := t.HandleUpdate(m)
			if err != nil {
				t.log.Error(err)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// HandleUpdate func
func (t *TokenStore) HandleUpdate(m Message) error {
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
	if val, ok := t.store[tid]; ok {
		return val, nil
	}
	return Token{}, fmt.Errorf("Token does not exist")
}

// Validate function
func (t *TokenStore) Validate(typ string, uid string, tid string) bool {
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
	for _, v := range t.store {
		if v.UID == uid {
			return v, nil
		}
	}
	return Token{}, fmt.Errorf("User does not have any issued tokens")
}

// Delete function
func (t *TokenStore) Delete(tid string) error {
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
	return fmt.Errorf("Token does not exist")
}

// Issue function
func (t *TokenStore) Issue(user string, typ string, expire ...bool) (Token, error) {
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
		IssuedBy:   t.app.Cluster.node.Addr(),
		Token:      token,
	}
	t.lock.Lock()
	t.store[token] = tok
	t.lock.Unlock()
	err := t.emit()
	if err != nil {
		return tok, err
	}
	return tok, nil
}

func (t *TokenStore) emit() error {
	t.lock.Lock()
	b, err := json.Marshal(t.store)
	if err != nil {
		return err
	}
	err = t.app.Cluster.Emit("token", b, "token:update")
	if err != nil {
		return err
	}
	return nil
}
