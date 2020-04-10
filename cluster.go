package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

//Cluster type
type Cluster struct {
	app           *Bunker
	terminate     chan bool
	config        *Config
	node          *noise.Node
	network       *kademlia.Protocol
	epoch         uint64
	updates       chan Message
	synced        chan bool
	peers         []noise.ID
	log           *Log
	locationTable []node
	genRSA        bool
	metrics       map[string]interface{}
}

func newCluster(app *Bunker) (*Cluster, error) {
	config := app.Config
	c := &Cluster{
		app:       app,
		config:    config,
		log:       app.Logger,
		terminate: make(chan bool),
		synced:    make(chan bool),
		genRSA:    false,
		metrics:   metrics(),
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
	)
	if err != nil {
		return c, err
	}
	c.node = node
	c.network = kademlia.New()
	c.node.Bind(c.network.Protocol())
	return c, nil
}

func metrics() map[string]interface{} {
	m := map[string]interface{}{
		"peers": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_cluster_network_size",
			Help: "The size of the peer network",
		}),
		"peerlatency": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bunker_cluster_peer_latency",
			Help: "Latencies in ms from this node to its peers",
		}, []string{"peer"}),
		"messages_rx": promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bunker_cluster_messages_rx",
			Help: "Number of cluster messages recieved by type",
		}, []string{"operation", "type"}),
		"messages_tx": promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bunker_cluster_messages_tx",
			Help: "Number of cluster messages sent by type",
		}, []string{"operation", "type"}),
		"sync_tx": promauto.NewCounter(prometheus.CounterOpts{
			Name: "bunker_cluster_sync_tx_bytes",
			Help: "Number of bytes transmitted in sync operations",
		}),
		"sync_rx": promauto.NewCounter(prometheus.CounterOpts{
			Name: "bunker_cluster_sync_rx_bytes",
			Help: "Number of bytes recieved in sync operations",
		}),
		"in": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_cluster_connections_inbound",
			Help: "Number of inbound cluster connections",
		}),
		"out": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_cluster_connections_outbound",
			Help: "Number of outbound cluster connections",
		}),
	}
	return m
}

func (c *Cluster) registerHandlers(updates chan Message, sync chan Message) error {
	if c.config.Mode == "dev" {
		return nil
	}
	c.node.Handle(func(ctx noise.HandlerContext) error {
		if ctx.IsRequest() {
			return nil
		}
		var msg Message
		err := json.Unmarshal(ctx.Data(), &msg)
		if err != nil {
			return err
		}
		go c.metrics["messages_rx"].(*prometheus.CounterVec).WithLabelValues(msg.Type, msg.DataType).Inc()
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
			if msg.DataType == "sync:sharedkey" {
				go func() {
					err := c.SendSharedKey(msg)
					if err != nil {
						c.log.Error(err)
					}
				}()
			}
			if msg.DataType == "sync:sendsharedkey" {
				go func() {
					err := c.HandleSharedKey(msg)
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
func (c *Cluster) Start(clusterReady chan bool) {
	if c.config.Mode == "dev" {
		c.genRSA = true
		clusterReady <- true
		return
	}
	startup := true
	firstNode := false
	if err := c.node.Listen(); err != nil {
		c.log.Fatal(err)
	}
	c.log.Debug("Start clustering")
	peered := false
	for peered == false {
		c.log.Debug("waiting for peers")
		select {
		case <-c.terminate:
			return
		default:
			// Wait for connection to our discovery host
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			if _, err := c.node.Ping(ctx, c.config.Cluster.DiscoveryHost); err == nil {
				peered = true
				cancel()
				break
			}
			cancel()
			time.Sleep(1 * time.Second)
			firstNode = true
		}
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
			go func() {
				c.metrics["peers"].(prometheus.Gauge).Set(float64(len(c.peers) + 1))
				c.metrics["in"].(prometheus.Gauge).Set(float64(len(c.node.Inbound())))
				c.metrics["out"].(prometheus.Gauge).Set(float64(len(c.node.Outbound())))
			}()
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
					go c.metrics["peerlatency"].(*prometheus.GaugeVec).WithLabelValues(p.Address).Set(float64(diff.Milliseconds()))
					cancel()
				}
				c.locationTable = ltab
				index = 0
			}
			if startup && !firstNode {
				err := c.RequestSharedKey()
				if err != nil {
					c.log.Error(err)
				}
				err = c.SyncRequest(clusterReady)
				if err != nil {
					c.log.Error(err)
					continue
				}
				startup = false
			}
			if startup && firstNode {
				c.genRSA = true
				clusterReady <- true
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
	go c.metrics["messages_tx"].(*prometheus.CounterVec).WithLabelValues(msg.Type, msg.DataType).Inc()
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
	exist := true
	if _, err := os.Stat(c.config.KV.DBPath); os.IsNotExist(err) {
		c.log.Warn("DB is empty, sending no data")
		exist = false
	}
	conn, err := net.Dial("tcp", string(msg.Data))
	if err != nil {
		return err
	}
	defer conn.Close()
	var db io.Reader
	if exist {
		db, err = os.OpenFile(c.config.KV.DBPath, os.O_RDONLY, 0755)
		if err != nil {
			return err
		}
	} else {
		db = bytes.NewBuffer([]byte{})
	}
	b, err := io.Copy(conn, db)
	if err != nil {
		return err
	}
	go c.metrics["sync_tx"].(prometheus.Counter).Add(float64(b))
	conn.Close()
	c.log.DebugF("Wrote %v bytes to sync operations", b)
	return nil
}

// SyncRequest emits a sync request to the nearest neighbor
// nearest is determined by the locationTable
func (c *Cluster) SyncRequest(clusterReady chan bool) error {
	c.log.Debug("New sync request")
	if c.config.Mode == "dev" {
		return nil
	}
	if len(c.locationTable) == 0 {
		return fmt.Errorf("No peers available to sync with")
	}
	c.log.Debug("At least 1 peer to sync with")
	id := uuid.New()
	syncAddress := fmt.Sprintf("%s:%v", strings.Split(c.node.ID().Address, ":")[0], c.config.Cluster.SyncPort)
	ready := make(chan error)
	go c.SyncHandle(syncAddress, ready, clusterReady)
	res := &Message{
		Epoch:    c.epoch + 1,
		Data:     []byte(syncAddress),
		DataType: "sync:request",
		Type:     "sync",
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	go c.metrics["messages_tx"].(*prometheus.CounterVec).WithLabelValues(res.Type, res.DataType).Inc()
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	// wait for TCP socket to open
	err = <-ready
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
	c.log.DebugF("Sending request to %s", c.locationTable[0].Address)
	err = c.node.Send(ctx, c.locationTable[0].Address, b)
	if err != nil {
		return err
	}
	return nil
}

// SyncHandle takes data from a syncReponse and
// writes the db to disk
func (c *Cluster) SyncHandle(addr string, ready chan error, clusterReady chan bool) {
	defer func() { clusterReady <- true }()
	srv, err := net.Listen("tcp", fmt.Sprintf(":%v", c.config.Cluster.SyncPort))
	if err != nil {
		ready <- err
		return
	}
	defer srv.Close()
	ready <- nil
	conn, err := srv.Accept()
	defer conn.Close()
	if err != nil {
		c.log.Error(err)
		return
	}
	err = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	if err != nil {
		c.log.Error(err)
		return
	}
	var buf bytes.Buffer
	n1, err := io.Copy(&buf, conn)
	if err != nil {
		c.log.Error(err)
		return
	}
	go c.metrics["sync_rx"].(prometheus.Counter).Add(float64(n1))
	c.log.DebugF("Got %v bytes in sync operation", n1)
	conn.Close()
	if c.app.KVInit {
		err := dbClose(c.app.KV.db)
		if err != nil {
			c.log.Error(err)
			return
		}
	}
	db, err := os.OpenFile(c.config.KV.DBPath, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		c.log.Error(err)
		return
	}
	defer db.Close()
	n2, err := io.Copy(db, &buf)
	if err != nil {
		c.log.Error(err)
		return
	}
	err = db.Sync()
	if err != nil {
		c.log.Error(err)
		return
	}
	c.log.DebugF("Copied %v bytes from tmp to db file", n2)
	if n1 != n2 {
		c.log.ErrorF("Got %v from sync but only wrote %v bytes to db", n1, n2)
		return
	}
	if c.app.KVInit {
		c.app.KV.db, err = dbOpen(c.app.KV.dbPath, c.app.KV.options)
		if err != nil {
			c.log.Error(err)
			return
		}
	}
	c.log.Debug("Synced database")
	return
}

// RequestSharedKey function
func (c *Cluster) RequestSharedKey() error {
	if c.config.Mode == "dev" {
		return nil
	}
	if len(c.locationTable) == 0 {
		return fmt.Errorf("No peers available to sync with")
	}
	c.log.Debug("At least 1 peer to sync with")
	id := uuid.New()
	res := &Message{
		Epoch:    c.epoch + 1,
		Data:     []byte{},
		DataType: "sync:sharedkey",
		Type:     "sync",
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	go c.metrics["messages_tx"].(*prometheus.CounterVec).WithLabelValues(res.Type, res.DataType).Inc()
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
	c.log.DebugF("Sending sharedkey request to %s", c.locationTable[0].Address)
	err = c.node.Send(ctx, c.locationTable[0].Address, b)
	if err != nil {
		return err
	}
	return nil
}

// SendSharedKey function
func (c *Cluster) SendSharedKey(msg Message) error {
	if c.config.Mode == "dev" {
		return nil
	}
	id := uuid.New()
	data, err := json.Marshal(c.app.sharedKey)
	if err != nil {
		return err
	}
	res := &Message{
		Epoch:    c.epoch + 1,
		Data:     data,
		DataType: "sync:sendsharedkey",
		Type:     "sync",
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	go c.metrics["messages_tx"].(*prometheus.CounterVec).WithLabelValues(res.Type, res.DataType).Inc()
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.log.DebugF("Sending sharedkey reply to %s", msg.Origin)
	err = c.node.Send(ctx, msg.Origin, b)
	if err != nil {
		return err
	}
	return nil
}

//HandleSharedKey function
func (c *Cluster) HandleSharedKey(msg Message) error {
	if c.config.Mode == "dev" {
		return nil
	}
	var key *AESKey
	err := json.Unmarshal(msg.Data, &key)
	if err != nil {
		return err
	}
	err = c.app.Crypto.SealSharedKey(key, c.app.Crypto.privkey, false)
	if err != nil {
		return err
	}
	c.log.Debug("Got shared key from " + msg.Origin)
	return nil
}
