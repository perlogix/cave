package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

//Cluster type
type Cluster struct {
	terminate     chan bool
	config        *Config
	node          *noise.Node
	network       *kademlia.Protocol
	epoch         uint64
	updates       chan Message
	synced        bool
	peers         []noise.ID
	log           *Log
	locationTable []node
}

func newCluster(app *Bunker) (*Cluster, error) {
	config := app.Config
	c := &Cluster{
		config:    config,
		log:       app.Logger,
		terminate: make(chan bool),
		synced:    false,
	}
	if c.config.Mode == "dev" {
		return c, nil
	}
	node, err := noise.NewNode(
		noise.WithNodeAddress(config.Cluster.AdvertiseHost),
		noise.WithNodeBindPort(config.Cluster.BindPort),
		noise.WithNodeIdleTimeout(300*time.Second),
		noise.WithNodeMaxInboundConnections(4096),
		noise.WithNodeMaxOutboundConnections(4096),
		noise.WithNodeMaxRecvMessageSize(1000000000),
	)
	if err != nil {
		return c, err
	}
	c.node = node
	c.network = kademlia.New()
	c.node.Bind(c.network.Protocol())
	return c, nil
}

func (c *Cluster) registerHandlers(updates chan Message, sync chan Message) error {
	c.node.Handle(func(ctx noise.HandlerContext) error {
		if ctx.IsRequest() {
			return nil
		}
		var msg Message
		err := json.Unmarshal(ctx.Data(), &msg)
		if err != nil {
			return err
		}
		c.log.Pretty(msg)
		switch msg.Type {
		case "update":
			updates <- msg
		case "sync":
			if msg.DataType == "sync:request" {
				go func() {
					err := c.SyncResponse(msg)
					if err != nil {
						c.log.Error(err)
					}
				}()
			}
			if msg.DataType == "sync:response" {
				go func() {
					err := c.SyncHandle(msg)
					if err != nil {
						c.log.Error(err)
					}
				}()
			}
		default:
			c.log.ErrorF("No channel for message type %s", msg.Type)
		}
		return nil
	})
	return nil
}

//Start starts the cluster
func (c *Cluster) Start() {
	startup := true
	firstNode := false
	if err := c.node.Listen(); err != nil {
		c.log.Fatal(err)
	}
	if c.config.Mode == "dev" {
		return
	}
	c.log.Debug("Start clustering")
	for {
		c.log.Debug("waiting for peers")
		select {
		case <-c.terminate:
			return
		default:
			// Wait for connection to our discovery host
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			if _, err := c.node.Ping(ctx, c.config.Cluster.DiscoveryHost); err == nil {
				cancel()
				break
			}
			cancel()
			time.Sleep(1 * time.Second)
			firstNode = true
			c.synced = true
		}
		break
	}
	c.log.Debug("Found at least 1 peer")
	index := 0
	for {
		// once we get a peer connection we can get the rest of the peers
		select {
		case <-c.terminate:
			c.log.Info("Got termination signal")
			return
		default:
			//c.log.Debug("Discovering network")
			c.peers = c.network.Discover()
			if index == 60 || index == 0 { // every 30 seconds or so
				ltab := []node{}
				for _, p := range c.peers {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					start := time.Now()
					_, err := c.node.Ping(ctx, p.Address)
					if err != nil {
						c.log.Error(err)
						cancel()
						continue
					}
					diff := time.Now().Sub(start)
					ltab = append(ltab, node{
						ID:       p.ID.String(),
						Address:  p.Address,
						Distance: diff,
					})
					cancel()
				}
				c.locationTable = ltab
				index = 0
			}
			if startup && !firstNode {
				err := c.SyncRequest()
				if err != nil {
					c.log.Error(err)
					continue
				}
				startup = false
			}
			time.Sleep(500 * time.Millisecond)
			index++
		}
	}
}

// Emit sends a message to the cluster
func (c *Cluster) Emit(typ string, data []byte, dtype string) error {
	if c.config.Mode == "dev" {
		return nil
	}
	id := uuid.New()
	msg := &Message{
		Epoch:    c.epoch + 1,
		Data:     data,
		DataType: dtype,
		Type:     typ,
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	msg.Epoch = c.epoch + 1
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	for _, p := range c.peers {
		go func(b []byte, p string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := c.node.Send(ctx, p, b)
			if err != nil {
				c.log.Error(err)
			}
		}(b, p.Address)
	}
	c.epoch++
	return nil
}

//SyncResponse syncs a cluster's kv store
func (c *Cluster) SyncResponse(msg Message) error {
	if c.config.Mode == "dev" {
		return nil
	}
	id := uuid.New()
	if _, err := os.Stat(c.config.KV.DBPath); os.IsNotExist(err) {
		return err
	}
	data, err := ioutil.ReadFile(c.config.KV.DBPath)
	if err != nil {
		return err
	}
	res := &Message{
		Epoch:    c.epoch + 1,
		Data:     data,
		DataType: "sync:response",
		Type:     "sync",
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	err = c.node.Send(ctx, msg.Origin, b)
	if err != nil {
		return err
	}
	c.epoch++
	return nil
}

// SyncRequest emits a sync request to the nearest neighbor
// nearest is determined by the locationTable
func (c *Cluster) SyncRequest() error {
	if c.config.Mode == "dev" {
		return nil
	}
	if len(c.locationTable) == 0 {
		return fmt.Errorf("No peers available to sync with")
	}
	id := uuid.New()
	res := &Message{
		Epoch:    c.epoch + 1,
		Data:     []byte{},
		DataType: "sync:request",
		Type:     "sync",
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(c.locationTable) > 1 {
		sort.Slice(c.locationTable, func(i, j int) bool {
			return c.locationTable[i].Distance < c.locationTable[j].Distance
		})
	}
	err = c.node.Send(ctx, c.locationTable[0].Address, b)
	if err != nil {
		return err
	}
	return nil
}

// SyncHandle takes data from a syncReponse and
// writes the db to disk
func (c *Cluster) SyncHandle(msg Message) error {
	if c.config.Mode == "dev" {
		return nil
	}
	err := ioutil.WriteFile(c.config.KV.DBPath, msg.Data, 0755)
	if err != nil {
		return err
	}
	c.synced = true
	return nil
}
