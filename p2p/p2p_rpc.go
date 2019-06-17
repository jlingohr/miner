package p2p

import (
	"errors"
	"fmt"
	"github.com/jlingohr/rfs-miner/bclib"
	"github.com/jlingohr/rfs-miner/mining"
	"github.com/jlingohr/rfs-miner/settings"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

const (
	MAX_BLOCK_SIZE = 100
)

type PeerRPC interface {
	DialPeer(peerInfo NewPeerInfo, resp *string) error
	FloodBlock(req FloodBlockReq, resp *struct{}) error
	FloodOperation(req FloodOpReq, resp *struct{}) error
	GetBlocks(hash []byte, resp *[]bclib.Block) error
	Heartbeat(hbeat int, hbeatResponse *bool) error
}

type MinerServer struct {
	clientRPC      *Server
	config         settings.Settings
	receivedBlocks chan FloodBlockReq
	receivedOps    chan bclib.Operation
	floodOperation chan FloodOpReq
	wg             sync.WaitGroup
	mine           chan struct{}
	stopMining     chan struct{}
	stop           chan struct{}

	newPeer            chan *Peer
	removePeers        chan string
	downloadPrevBlocks chan bclib.Block
	downloadedBlocks   chan []bclib.Block
	downloadErrs       chan error
	peerErrs           chan PeerError

	manager *ChainManager
}

type NewPeerInfo struct {
	Address    string
	MinerID    string
	Blockchain []bclib.Block
}

type BlockDoesNotExistError string

func (e BlockDoesNotExistError) Error() string {
	return fmt.Sprintf("The blockchain does not contain a Block with hash [%s]", string(e))
}

type NewPeerResponse struct {
	MinerID    string
	Blockchain []bclib.Block
}

type NewPeerRequest struct {
	peer *Peer
	resp chan []bclib.Block
}

func (ms *MinerServer) DialPeer(info NewPeerInfo, resp *NewPeerResponse) error {
	//TODO sync blockchain
	addr, err := net.ResolveTCPAddr("tcp", info.Address)
	if err != nil {
		return err
	}

	conn, err := rpc.Dial("tcp", addr.String())
	if err != nil {
		return err
	}

	newPeer := NewPeer(info.MinerID, addr.String(), conn, ms.downloadedBlocks, ms.downloadErrs, ms.peerErrs)
	ms.newPeer <- newPeer

	if len(info.Blockchain) > 1 {
		syncBlockchain := SynchronizeBlockchain{info.Blockchain, make(chan struct{}), make(chan error)}
		ms.manager.syncBlockchain <- syncBlockchain
		select {
		case <-syncBlockchain.done:
			ms.manager.mux.RLock()
			response := NewPeerResponse{
				MinerID:    ms.config.MinerID,
				Blockchain: ms.manager.blockchain.GetBlocks(),
			}
			ms.manager.mux.RUnlock()

			*resp = response

			return nil
		case err := <-syncBlockchain.err:
			return err
		}
	}

	ms.manager.mux.RLock()
	response := NewPeerResponse{
		MinerID:    ms.config.MinerID,
		Blockchain: ms.manager.blockchain.GetBlocks(),
	}
	ms.manager.mux.RUnlock()

	*resp = response

	return nil
}

type FloodBlockReq struct {
	Block       bclib.Block
	FromMinerId string
}

type NewFloodBlockReq struct {
	FloodBlockReq
	received chan struct{}
}

type MakeFloodBlockReq struct {
	FloodBlockReq
	ignorePeers map[string]struct{}
}

func (ms *MinerServer) FloodBlock(req FloodBlockReq, resp *struct{}) error {
	// todo handle loops

	block := req.Block

	if block.Operations == nil {
		block.Operations = make([]bclib.Operation, 0)
	}

	ms.receivedBlocks <- req
	//}
	*resp = struct{}{}
	return nil
}

type FloodOpReq struct {
	Operation    bclib.Operation
	IgnoreMiners []string
}

func (ms *MinerServer) FloodOperation(req FloodOpReq, resp *struct{}) error {
	log.Printf("Peer %s received an operation %s with ignorePeers = %v", ms.config.MinerID, req.Operation.OperationID, req.IgnoreMiners)
	*resp = struct{}{}
	ms.floodOperation <- req
	ms.receivedOps <- req.Operation

	return nil
}

func (ms *MinerServer) GetBlocks(hash []byte, resp *[]bclib.Block) error {
	blocks := ms.manager.GetPreviousBlocks(hash, 500)

	if len(blocks) == 0 {
		return errors.New(fmt.Sprintf("%s does not have blocks for hash %x", ms.config.MinerID, hash))
	}
	*resp = blocks

	return nil
}

type PendingBlock struct {
	originMinerId string
	block         bclib.Block
}

func (ms *MinerServer) startListening() error {
	handler := rpc.NewServer()
	err := handler.Register(ms)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", ms.config.IncomingMinersAddr)
	if err != nil {
		return err
	}

	log.Println("MinerRPC Waiting for connections")

	listen := func(listener net.Listener) <-chan net.Conn {
		newConn := make(chan net.Conn)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err)
				newConn <- nil
			}
			newConn <- conn
		}()

		return newConn
	}

	ms.wg.Add(1)
	go func() {
		for {
			select {
			case <-ms.stop:
				listener.Close()
				ms.wg.Done()
				return
			case conn := <-listen(listener):
				if conn != nil {
					log.Printf("%s got connection from %s", ms.config.MinerID, conn.RemoteAddr())
					go handler.ServeConn(conn)
				}
			}
		}
	}()

	return nil
}

func NewMinerServer(config settings.Settings) (*MinerServer, error) {
	bc := bclib.NewBlockchain(config)
	log.Printf("Miner %s created new blockchain using genesis %s", config.MinerID, config.GenesisBlockHash)
	manager := NewChainManager(bc, config)

	ms := &MinerServer{
		config:             config,
		wg:                 sync.WaitGroup{},
		stop:               make(chan struct{}),
		receivedOps:        make(chan bclib.Operation),
		floodOperation:     make(chan FloodOpReq),
		receivedBlocks:     make(chan FloodBlockReq),
		mine:               make(chan struct{}, 1),
		stopMining:         make(chan struct{}),
		downloadPrevBlocks: make(chan bclib.Block),

		newPeer:          make(chan *Peer),
		removePeers:      make(chan string),
		downloadedBlocks: make(chan []bclib.Block),
		downloadErrs:     make(chan error),
		peerErrs:         make(chan PeerError),

		manager: manager,
	}

	ms.wg.Add(1)
	go ms.managePeers()

	s, err := NewRPCServer(ms, manager)
	if err != nil {
		return nil, err
	}

	ms.clientRPC = s
	ms.manager.newBlockAdded = s.newBlockAdded
	ms.manager.downloadBlock = ms.downloadPrevBlocks

	err = s.startListening()
	if err != nil {
		return nil, err
	}

	ms.wg.Add(1)
	go ms.startMining()

	err = ms.startMinerRPC()
	if err != nil {
		return nil, err
	}

	return ms, nil
}

func (ms *MinerServer) startMining() {
	defer ms.wg.Done()
	log.Printf("%s starting miner thread", ms.config.MinerID)

	minerId := ms.config.MinerID
	powPerNoOpBlock := ms.config.PowPerNoOpBlock
	powPerOpBlock := ms.config.PowPerOpBlock

	batchedOperations := ms.buildPendingOperations()
	finishedMining := make(chan bclib.Block)
	isMining := false

	worker := mining.NewWorker(minerId, powPerNoOpBlock, powPerOpBlock, ms.manager, finishedMining)

	//ms.mine <- struct{}{}

	for {
		select {
		case <-ms.mine:
			if isMining {
				continue
			}

			isMining = true
			go worker.StartMining(batchedOperations)
		case block := <-finishedMining:
			if block.Nonce > 0 {
				isMining = false
				ms.manager.addBlock <- block
			}
		case <-ms.stopMining:
			if worker != nil {
				log.Printf("Node %s stopping worker", minerId)
				worker.Stop()
				worker = nil
			}
		case <-ms.stop:
			if worker != nil {
				worker.Stop()
				return
			}
		}
	}
}

func (ms *MinerServer) shutdown() {
	close(ms.stop)
	ms.wg.Wait()
}

func (ms *MinerServer) buildPendingOperations() <-chan []bclib.Operation {
	finalizedOperations := make(chan []bclib.Operation, 10)

	go func() {
		defer close(finalizedOperations)
		operations := make(map[string]bclib.Operation, MAX_BLOCK_SIZE)
		var ticker *time.Ticker
		var C <-chan time.Time

		for {
			select {
			case <-ms.stop:
				return
			case op := <-ms.receivedOps:
				log.Printf("%s RECEIVED AN OPERATION IN RECEIVE OPS", ms.config.MinerID)
				_, duplicateOp := operations[op.OperationID]

				if duplicateOp {
					continue
				}
				// Return false to a client if the client submits an append operation
				// for a file that does not exist, or if the client submits a create file
				// operation for a file that already exists (both cases assume confirmed ops)
				ms.clientRPC.mux.Lock()
				_, ok := ms.clientRPC.confirmedFiles[op.Filename]
				respChan, pendingClientOperation := ms.clientRPC.clientOps[op.OperationID]

				if (ok && op.Op == bclib.Create) || (!ok && op.Op == bclib.Append) {
					if pendingClientOperation {
						respChan.Ready <- false
						delete(ms.clientRPC.clientOps, op.OperationID)
						continue
					}
				}
				ms.clientRPC.mux.Unlock()

				//TODO Verify that the code below is correct
				// If invalid because no coins and from out client then drop the operation
				// If invalid and not consistent with blockchain and from our client then return to clientRPC client map false
				_, valid := ms.manager.ValidateOperations([]bclib.Operation{op}, ms.manager.GetLastHash())

				if !valid {
					continue // drop op
				}

				operations[op.OperationID] = op

				if pendingClientOperation {
					ms.floodOperation <- FloodOpReq{
						IgnoreMiners: make([]string, 0),
						Operation:    op,
					}
				}

				if len(operations) == 1 {
					ticker = time.NewTicker(time.Duration(ms.config.GenOpBlockTimeout) * time.Second)
					C = ticker.C
				}
				if len(operations) == MAX_BLOCK_SIZE {
					ticker.Stop()
					finalizedOperations <- bclib.ListOpsFromMap(operations)
					operations = make(map[string]bclib.Operation, MAX_BLOCK_SIZE)
				}
			case <-C:
				ticker.Stop()
				finalizedOperations <- bclib.ListOpsFromMap(operations)
				operations = make(map[string]bclib.Operation, MAX_BLOCK_SIZE)
				C = nil
			}
		}
	}()

	return finalizedOperations
}

func (ms *MinerServer) managePeers() {
	defer ms.wg.Done()

	ticker := time.NewTicker(5 * time.Second)

	peers := make(map[string]*Peer)
	blocksToPeers := make(map[string][]string) //map which peers have sent us which blocks

	for {
		select {
		case <-ticker.C:
			log.Println("ManagePeers Heartbeat")
		case <-ms.stop:
			log.Printf("%s closing all peer connections", ms.config.MinerID)
			for _, peer := range peers {
				peer.Conn.Close()
			}
			return
		case newPeer := <-ms.newPeer:
			peers[newPeer.MinerID] = newPeer
			log.Printf("%s added peer %s", ms.config.MinerID, newPeer.MinerID)
			ms.mine <- struct{}{}
			ms.clientRPC.havePeers <- len(peers) > 0
		case <-ms.manager.continueMining:
			if len(peers) > 0 {
				ms.mine <- struct{}{}
			}
		case req := <-ms.receivedBlocks:
			fromID := req.FromMinerId
			block := req.Block

			miners, ok := blocksToPeers[string(block.Hash)]
			if !ok {
				blocksToPeers[string(block.Hash)] = []string{fromID}
			} else {
				blocksToPeers[string(block.Hash)] = append(miners, fromID)
			}

			ms.manager.receiveFloodedBlock <- block

		case block := <-ms.downloadPrevBlocks:
			log.Printf("%s trying to download %x", ms.config.MinerID, block.PreviousHash)
			miners, ok := blocksToPeers[string(block.Hash)]
			if ok {
				for _, minerID := range miners {
					peer, ok := peers[minerID]
					if ok {
						peer.requestBlocks <- block.PreviousHash
						select {
						case blocks := <-ms.downloadedBlocks:
							ms.manager.downloadedBlocks <- blocks
							log.Println("SENT DOWNLOADED BLOCKS OVER THE CHAN")
						case err := <-ms.downloadErrs:
							log.Printf("%s downloading blocks from %s - %s", ms.config.MinerID, peer.MinerID, err)
							continue
						}
					}
				}
			}

		case block := <-ms.manager.sendBlock:
			ignorePeers := make(map[string]struct{})

			miners, ok := blocksToPeers[string(block.Hash)]
			if ok {
				for _, minerID := range miners {
					ignorePeers[minerID] = struct{}{}
				}
			}

			req := FloodBlockReq{
				Block:       block,
				FromMinerId: ms.config.MinerID,
			}

			for peerMinerId := range peers {
				_, ignore := ignorePeers[peerMinerId]
				if !ignore {
					peer, ok := peers[peerMinerId]
					if !ok {
						continue
					}
					peer.sendBlock <- req
					//todo maybe return to manager that flood is successful
				}
			}
			log.Printf("%s flooded Block with hash %x to %x", ms.config.MinerID, req.Block.Hash)

		case op := <-ms.floodOperation:
			ignorePeers := make(map[string]struct{})

			// Add peers that have already received the op to the set
			for _, ignoreId := range op.IgnoreMiners {
				ignorePeers[ignoreId] = struct{}{}
			}

			// Filter out peers that we need to flood to
			peersToFloodTo := make([]*Peer, 0)

			for _, peer := range peers {
				_, ignore := ignorePeers[peer.MinerID]
				if !ignore {
					peersToFloodTo = append(peersToFloodTo, peer)
				}
			}

			// Add our peers to the set, since we will send them the operation
			for peerId := range peers {
				ignorePeers[peerId] = struct{}{}
			}

			ignorePeers[ms.config.MinerID] = struct{}{}

			opFloodReq := FloodOpReq{
				IgnoreMiners: StringSetToList(ignorePeers),
				Operation:    op.Operation,
			}

			for _, peer := range peersToFloodTo {
				peer.sendOp <- opFloodReq
			}
			log.Printf("%s flooded operation %s", ms.config.MinerID, opFloodReq.Operation.OperationID)

		case err := <-ms.peerErrs:
			log.Printf("%s removing peer %s - %s", ms.config.MinerID, err.minerID, err.err)
			peer, ok := peers[err.minerID]
			if !ok {
				continue
			}
			close(peer.done)
			delete(peers, err.minerID)
			ms.clientRPC.havePeers <- len(peers) > 0
		}
	}
}

type PeerError struct {
	minerID string
	err     error
}

func StringSetToList(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))

	for value := range set {
		result = append(result, value)
	}

	return result
}

type SynchronizeBlockchain struct {
	PeerBlocks []bclib.Block
	done       chan struct{}
	err        chan error
}

func (ms *MinerServer) startMinerRPC() error {
	err := ms.startListening()
	if err != nil {
		return err
	}

	dialedPeers := make(map[string]struct{})
	for len(dialedPeers) < len(ms.config.PeerMinersAddrs) {
		for _, addr := range ms.config.PeerMinersAddrs {
			if _, ok := dialedPeers[addr]; ok {
				continue
			}
			//log.Printf("%s dialing %s", ms.config.MinerID, addr)
			conn, err := rpc.Dial("tcp", addr)
			if err != nil {
				//log.Println(err)
				continue //todo discover later
			}

			ms.manager.mux.RLock()
			info := NewPeerInfo{Address: ms.config.IncomingMinersAddr, MinerID: ms.config.MinerID, Blockchain: ms.manager.blockchain.GetBlocks()}
			ms.manager.mux.RUnlock()

			var response NewPeerResponse
			err = conn.Call("MinerServer.DialPeer", info, &response)
			if err != nil {
				//log.Println(err)
				continue
			}

			newPeer := NewPeer(response.MinerID, addr, conn, ms.downloadedBlocks, ms.downloadErrs, ms.peerErrs)
			ms.newPeer <- newPeer

			// Obtain dialed peers blockchain and send to blockchain manager to synchronize
			syncBlockchainRequest := SynchronizeBlockchain{response.Blockchain, make(chan struct{}), make(chan error)}
			ms.manager.syncBlockchain <- syncBlockchainRequest
			select {
			//Wait for synchronization to complete before dialing another peer
			case <-syncBlockchainRequest.done:
				log.Printf("%s synchronized with %s", ms.config.MinerID, newPeer.MinerID)
				dialedPeers[addr] = struct{}{}
			case err := <-syncBlockchainRequest.err:
				log.Println(err)
				continue
			}
		}
	}

	log.Printf("%s finished dialing peers", ms.config.MinerID)

	return nil
}

func (ms *MinerServer) Heartbeat(hbeat int, hbeatResponse *bool) error {
	*hbeatResponse = true
	return nil
}
