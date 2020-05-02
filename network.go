package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/ethereum/go-ethereum/rpc"
)

// Network defines the DHT network
type Network struct {
	app       *Bunker
	Options   NetworkOptions
	rpcServer *rpc.Server
	rpcClient *rpc.Client
	Node      *Node
}

// TCPNetwork type
type TCPNetwork struct {
	server net.Listener
}

// NetworkOptions type
type NetworkOptions struct {
	AnnounceHost   string
	Port           uint16
	IP             string
	MaxMessageSize uint64 // max size in bytes
	LogFunc        func(string, string)
	App            *Bunker
	Certificate    string
	CertificateKey string
}

// Peer type
type Peer struct {
	Addr     string `json:"addr"`     // host:port
	ID       string `json:"id"`       // machineID
	Distance int64  `json:"distance"` // distance in ns
}

// ClusterMessage type
type ClusterMessage struct {
	Type     string
	TxMethod string // direct, broadcast
	Source   Peer
	Body     json.RawMessage
}

// NewNetwork func
func NewNetwork(app *Bunker, opts NetworkOptions) (*Network, error) {
	id, err := machineid.ID()
	if err != nil {
		return nil, err
	}
	self := Peer{
		ID:       id,
		Addr:     fmt.Sprintf("%s:%v", opts.IP, opts.Port),
		Distance: 0,
	}
	return &Network{
		app:     app,
		Options: opts,
		Node: &Node{
			crypto: opts.App.Crypto,
			lock:   sync.Mutex{},
			Self:   self,
			Peers: map[string]Peer{
				id: self,
			},
		},
	}, nil
}

//Start function
func (n *Network) Start() error {
	if n.app.Config.Mode == "dev" {
		return nil
	}
	n.rpcServer = rpc.NewServer()
	err := n.RegisterService("node", n.Node)
	if err != nil {
		return err
	}
	http.HandleFunc("/rpc", n.rpcServer.ServeHTTP)
	go http.ListenAndServeTLS(fmt.Sprintf("%s:%v", n.Options.IP, n.Options.Port), n.Options.Certificate, n.Options.CertificateKey, nil)
	i := 0
	for i < 120 {
		i++
		var p map[string]Peer
		err = n.CallOne(n.Options.AnnounceHost, "node_announce", &p, n.Node.Self)
		if err != nil {
			fmt.Println(err)
			time.Sleep(1 * time.Second)
			continue
		}
		for _, i := range p {

			n.Node.Peers[i.ID] = i

		}
		break
	}
	go n.checkPeers()
	return nil
}

func (n *Network) checkPeers() {
	t := time.NewTicker(10 * time.Second)
	for range t.C {
		fmt.Println("Starting peer checking")
		err := n.syncPeers()
		if len(err) != 0 {
			for _, e := range err {
				n.Options.LogFunc("error", e.Error())
			}
		}

		nodes := n.Node.List()

		var errs []error
		for k, v := range nodes {
			if k == n.Node.Self.ID {
				continue
			}
			start := time.Now()
			res := false
			err := n.CallOne(v.Addr, "node_heartbeat", &res)
			if err != nil {
				errs = append(errs, err)
			}
			if res {
				fmt.Printf("Peer %s OK", v.Addr)
				r := v
				r.Distance = time.Since(start).Nanoseconds()

				n.Node.Peers[k] = r

			} else {
				fmt.Printf("Peer %s ERR", v.Addr)

				delete(n.Node.Peers, k)

			}
		}
		if len(errs) != 0 {
			for _, e := range errs {
				n.Options.LogFunc("error", e.Error())
			}
		}
		n.app.Logger.Pretty(n.Node.Peers)
	}
}

func (n *Network) syncPeers() []error {
	fmt.Println("start peer sync")
	newList := map[string]Peer{}
	dest := []interface{}{}
	err := n.CallQuorum("node_list", &dest)
	if len(err) != 0 {
		return err
	}
	fmt.Println(dest)
	for _, l := range dest {
		for _, p := range l.([]Peer) {
			newList[p.ID] = p
		}
	}

	n.Node.Peers = newList

	n.app.Logger.Pretty(n.Node.Peers)
	return []error{}
}

// RegisterService allows you to register a new service
func (n *Network) RegisterService(name string, svc interface{}) error {
	return n.rpcServer.RegisterName(name, svc)
}

// CallOne calls the specified host
func (n *Network) CallOne(host string, serviceName string, dest interface{}, args ...interface{}) error {
	return n.call(host, serviceName, dest, args...)
}

// CallAny calls the closest node in the table
func (n *Network) CallAny(serviceName string, dest interface{}, args ...interface{}) error {
	p := n.getNearestNeighbor()
	if p.Addr == "" {
		return fmt.Errorf("No peers available")
	}
	return n.call(p.Addr, serviceName, dest, args...)
}

// CallQuorum calls 1/2n+1 nodes when len(nodes) < 5
func (n *Network) CallQuorum(serviceName string, dest *[]interface{}, args ...interface{}) []error {
	var errs []error
	peerList := []Peer{}
	sortedList := []Peer{}
	for _, v := range n.Node.Peers {
		if v.ID == n.Node.Self.ID {
			continue
		}
		peerList = append(peerList, v)
	}
	if len(peerList) <= 5 {
		sortedList = append(sortedList, peerList...)
	}
	if len(peerList) > 5 {
		sort.Slice(peerList, func(i, j int) bool {
			return peerList[i].Distance < peerList[j].Distance
		})
		for i := 0; i <= len(peerList)/2+1; i++ {
			sortedList = append(sortedList, peerList[i])
		}
	}
	var dst []interface{}
	for _, p := range sortedList {
		var d interface{}
		err := n.call(p.Addr, serviceName, &d, args...)
		if err != nil {
			errs = append(errs, err)
		}
		dst = append(dst, d)
	}
	*dest = dst
	return nil
}

// CallAll calls every node in the table
func (n *Network) CallAll(serviceName string, dest *[]interface{}, args ...interface{}) []error {
	var errs []error
	var dst []interface{}
	for _, p := range n.Node.Peers {
		if p.ID == n.Node.Self.ID {
			continue
		}
		var d interface{}
		err := n.call(p.Addr, serviceName, &d, args...)
		if err != nil {
			errs = append(errs, err)
		}
		dst = append(dst, d)
	}
	*dest = dst
	return nil
}

func (n *Network) call(host string, serviceName string, dest interface{}, args ...interface{}) error {
	url := "https://" + host + "/rpc"
	fmt.Printf("Calling %s on %s with args %+v\n", serviceName, url, args)
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: n.app.Config.Cluster.IgnoreSSLErrors,
			},
		},
	}
	conn, err := rpc.DialHTTPWithClient(url, hc)
	if err != nil {
		fmt.Println("Error with DIAL: ", err)
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = conn.CallContext(ctx, &dest, serviceName, args...)
	if err != nil {
		fmt.Println("Error with CALL: ", err)
		return err
	}
	return nil
}

func (n *Network) getNearestNeighbor() Peer {
	var peerList []Peer
	for _, v := range n.Node.Peers {
		if v.ID == n.Node.Self.ID {
			continue
		}
		peerList = append(peerList, v)
	}
	if len(peerList) > 0 {
		sort.Slice(peerList, func(i, j int) bool {
			return peerList[i].Distance < peerList[j].Distance
		})
		for _, v := range peerList {
			if v.ID != n.Node.Self.ID {
				return v
			}
			continue
		}
	}
	return Peer{}
}

// Node type
type Node struct {
	lock   sync.Mutex
	Peers  map[string]Peer
	Self   Peer
	crypto *Crypto
}

// List lists peers
func (n *Node) List() map[string]Peer {
	n.lock.Lock()
	n.Peers[n.Self.ID] = n.Self
	n.lock.Unlock()
	return n.Peers
}

// Heartbeat func
func (n *Node) Heartbeat() bool {
	return true
}

// Announce func
func (n *Node) Announce(p Peer) map[string]Peer {
	n.lock.Lock()
	n.Peers[p.ID] = p
	n.lock.Unlock()
	return n.Peers
}

// GetSharedKey func
func (n *Node) GetSharedKey() (*AESKey, error) {
	key, err := n.crypto.UnsealSharedKey(n.crypto.privkey)
	if err != nil {
		return nil, err
	}
	return key, err
}
