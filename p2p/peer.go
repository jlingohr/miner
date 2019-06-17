package p2p

import (
	"github.com/jlingohr/rfs-miner/bclib"
	"log"
	"net/rpc"
	"time"
	//"net"
)

type PeerStatus uint8

const (
	Connected PeerStatus = iota
	Disconnected
)

type DisconnectReason uint8

const (
	ShuttingDown DisconnectReason = iota
	RequestedDisconnect
)

func (r DisconnectReason) String() string {
	var reason string
	switch r {
	case ShuttingDown:
		reason = "requested shutdown"
	}
	return reason
}

func (r DisconnectReason) Error() string {
	return r.String()
}

type Peer struct {
	MinerID       string
	Address       string
	Conn          *rpc.Client
	sendBlock     chan FloodBlockReq
	sendOp        chan FloodOpReq
	requestBlocks chan []byte
	done          chan struct{}
}

func NewPeer(minerID string, address string, conn *rpc.Client, downloadedBlocks chan<- []bclib.Block, downloadErrs chan<- error, errs chan<- PeerError) *Peer {
	p := &Peer{
		MinerID:       minerID,
		Address:       address,
		Conn:          conn,
		sendBlock:     make(chan FloodBlockReq),
		sendOp:        make(chan FloodOpReq),
		requestBlocks: make(chan []byte),
		done:          make(chan struct{}),
	}

	go p.run(downloadedBlocks, downloadErrs, errs)
	return p
}

func (p *Peer) run(downloadedBlocks chan<- []bclib.Block, downloadErrs chan<- error, errs chan<- PeerError) {
	ticker := time.NewTicker(5 * time.Second)
	lastConnected := time.Now()
	for {
		select {
		case req := <-p.sendBlock:
			var resp struct{}
			err := p.Conn.Call("MinerServer.FloodBlock", req, &resp)

			if err != nil {
				errs <- PeerError{minerID: p.MinerID, err: err}
				continue
			}
			lastConnected = time.Now()

		case req := <-p.sendOp:
			var resp struct{}
			err := p.Conn.Call("MinerServer.FloodOperation", req, &resp)
			if err != nil {
				errs <- PeerError{minerID: p.MinerID, err: err}
				continue
			}
			lastConnected = time.Now()

		case hash := <-p.requestBlocks:
			log.Println("INSIDE THE CALL")
			var blocks []bclib.Block
			err := p.Conn.Call("MinerServer.GetBlocks", hash, &blocks)
			if err != nil {
				downloadErrs <- err
			} else {
				downloadedBlocks <- blocks
				log.Println("DOWNLOADED THE BLOCKS SENDING OVER THE CHAN")
				lastConnected = time.Now()
			}

		case <-ticker.C:
			log.Printf("Peer %s heartbeat", p.MinerID)
			now := time.Now()
			if lastConnected.Add(5 * time.Second).Before(now) {
				var connected bool
				err := p.Conn.Call("MinerServer.Heartbeat", 0, &connected)
				if err != nil {
					errs <- PeerError{minerID: p.MinerID, err: err}
					continue
				}
				lastConnected = time.Now()
			}

		case <-p.done:
			p.Conn.Close()
			ticker.Stop()
			return
		}
	}
}
