package p2p

import (
	"github.com/jlingohr/rfs-miner/bclib"
	"github.com/jlingohr/rfs-miner/rfslib"
	"log"
	"math"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type RPCServer interface {
	CreateFile(filename string, resp *bool) error
	AppendRecord(recInfo rfslib.AppendRecordInfo, resp *int) error
	ReadRecord(recInfo rfslib.ReadRecordInfo, resp *rfslib.Record) error
	TotalRecs(fname string, resp *int) error
	ListFiles(req int, resp *[]string) error
}

type PendingOperation struct {
	operation bclib.Operation
	Ready     chan bool
}

type Server struct {
	Address *net.TCPAddr
	bc      bclib.RFSLedger
	mux     sync.RWMutex
	cmux    sync.RWMutex
	minerID string

	confirmedFiles map[string]struct{}
	clientOps      map[string]PendingOperation

	newBlockAdded   chan struct{}
	updateFileCache chan struct{}
	receivedOp      chan bclib.Operation
	havePeers       chan bool
	isConnected     bool
}

func (s *Server) ReadRecord(recInfo rfslib.ReadRecordInfo, resp *rfslib.Record) error {
	s.cmux.RLock()
	connected := s.isConnected
	s.cmux.RUnlock()

	if !connected {
		return rfslib.DisconnectedError(s.minerID)
	}

	_, ok := s.confirmedFiles[recInfo.Fname]

	if !ok {
		return rfslib.FileDoesNotExistError(recInfo.Fname)
	}

	for {
		file, ok := s.bc.GetFile(recInfo.Fname)

		if !ok {
			return rfslib.FileDoesNotExistError(recInfo.Fname)
		}

		if recInfo.RecNum < len(file.Operations) {
			var record rfslib.Record

			data := file.Operations[recInfo.RecNum].Data
			copy(record[:], data)

			*resp = record
			return nil
		}

		time.Sleep(10 * time.Second)
	}

	return nil
}

func (s *Server) AppendRecord(recInfo rfslib.AppendRecordInfo, resp *int) error {
	s.cmux.RLock()
	connected := s.isConnected
	s.cmux.RUnlock()

	if !connected {
		return rfslib.DisconnectedError(s.minerID)
	}

	s.mux.RLock()
	_, ok := s.confirmedFiles[recInfo.Fname]
	s.mux.RUnlock()

	if !ok {
		return rfslib.FileDoesNotExistError(recInfo.Fname)
	}

	file, ok := s.bc.GetFile(recInfo.Fname)
	if ok {
		if len(file.Operations) > math.MaxUint16 {
			return rfslib.FileMaxLenReachedError(recInfo.Fname)
		}
	}

	op := bclib.NewOperation(bclib.Append, s.minerID, recInfo.Fname, recInfo.Data[:])
	ready := make(chan bool, 10)
	s.mux.Lock()
	s.clientOps[op.OperationID] = PendingOperation{op, ready}
	s.mux.Unlock()
	s.receivedOp <- op
	success := <-ready

	if !success {
		return rfslib.FileDoesNotExistError(recInfo.Fname)
	}

	file, ok = s.bc.GetFile(recInfo.Fname)

	if !ok {
		return rfslib.FileDoesNotExistError(recInfo.Fname)
	}

	for i := len(file.Operations) - 1; i >= 0; i-- {
		if file.Operations[i].OperationID == op.OperationID {
			*resp = i
			return nil
		}
	}

	return nil
}

func (s *Server) ListFiles(req int, resp *[]string) error {
	s.cmux.RLock()
	connected := s.isConnected
	s.cmux.RUnlock()

	if !connected {
		return rfslib.DisconnectedError(s.minerID)
	}

	fnames := make([]string, 0, len(s.confirmedFiles))

	s.mux.RLock()
	for fname := range s.confirmedFiles {
		fnames = append(fnames, fname)
	}
	s.mux.RUnlock()

	*resp = fnames

	return nil
}

func (s *Server) TotalRecs(fname string, resp *int) error {
	s.cmux.RLock()
	connected := s.isConnected
	s.cmux.RUnlock()

	if !connected {
		return rfslib.DisconnectedError(s.minerID)
	}

	s.mux.RLock()
	_, ok := s.confirmedFiles[fname]
	s.mux.RUnlock()

	if !ok {
		return rfslib.FileDoesNotExistError(fname)
	}

	file, ok := s.bc.GetFile(fname)

	if !ok {
		return rfslib.FileDoesNotExistError(fname)
	}

	*resp = len(file.Operations)

	return nil
}

func (s *Server) CreateFile(filename string, resp *bool) error {
	s.cmux.RLock()
	connected := s.isConnected
	s.cmux.RUnlock()

	if !connected {
		return rfslib.DisconnectedError(s.minerID)
	}

	if s.conflictPendingCreate(filename) {
		return rfslib.FileExistsError(filename)
	}

	op := bclib.NewOperation(bclib.Create, s.minerID, filename, []byte{})
	ready := make(chan bool, 10)
	s.mux.Lock()
	s.clientOps[op.OperationID] = PendingOperation{op, ready}
	s.mux.Unlock()
	s.receivedOp <- op
	success := <-ready
	*resp = success

	if !success {
		return rfslib.FileExistsError(filename)
	}

	return nil
}

func (s *Server) updateConfirmedFiles() {
	for range s.updateFileCache {
		log.Println("Updating confirmed files")
		files := s.bc.ListFiles()
		confirmedFiles := make(map[string]struct{})
		for _, fname := range files {
			confirmedFiles[fname] = struct{}{}
		}
		s.mux.Lock()
		s.confirmedFiles = confirmedFiles
		s.mux.Unlock()
	}
}

func (s *Server) checkConfirmedOperations() {
	for range s.newBlockAdded {
		log.Println("Checking confirmed operations")
		removeOps := make([]PendingOperation, 0)
		s.mux.RLock()
		for _, op := range s.clientOps {
			confirmed, exists, otherMiner := s.bc.CheckOpConfirmed(op.operation)
			if confirmed && !otherMiner {
				removeOps = append(removeOps, op)
				op.Ready <- true
			} else if confirmed && otherMiner {
				removeOps = append(removeOps, op)
				op.Ready <- false
			}
			if !exists {
				s.receivedOp <- op.operation
			}
		}
		s.mux.RUnlock()

		s.mux.Lock()
		for _, op := range removeOps {
			delete(s.clientOps, op.operation.OperationID)
		}
		s.mux.Unlock()

		s.updateFileCache <- struct{}{}
	}
}

func (s *Server) conflictPendingCreate(fname string) (conflicts bool) {
	s.mux.RLock()
	for _, pendingOp := range s.clientOps {
		if pendingOp.operation.Op == bclib.Create {
			conflicts = pendingOp.operation.Filename == fname
		}
	}
	s.mux.RUnlock()

	return
}

func (s *Server) updateConnectedStatus() {
	for isConnected := range s.havePeers {
		s.cmux.Lock()
		s.isConnected = isConnected
		s.cmux.Unlock()
	}
}

func (s *Server) startListening() error {

	fnames := s.bc.ListFiles()
	for _, fname := range fnames {
		s.confirmedFiles[fname] = struct{}{}
	}

	handler := rpc.NewServer()
	err := handler.Register(s)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", s.Address.String())
	if err != nil {
		return err
	}

	log.Printf("Miner %s listening for clients on %s", s.minerID, s.Address.String())

	go s.updateConfirmedFiles()
	go s.checkConfirmedOperations()
	go s.updateConnectedStatus()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			log.Println("Serving client")
			go handler.ServeConn(conn)
		}
	}()
	return nil
}

func NewRPCServer(ms *MinerServer, bc bclib.RFSLedger) (*Server, error) {
	addr, err := net.ResolveTCPAddr("tcp", ms.config.IncomingClientsAddr)
	if err != nil {
		return nil, err
	}

	confirmedFiles := make(map[string]struct{})

	s := &Server{
		Address:         addr,
		minerID:         ms.config.MinerID,
		receivedOp:      ms.receivedOps,
		bc:              bc,
		confirmedFiles:  confirmedFiles,
		clientOps:       make(map[string]PendingOperation),
		updateFileCache: make(chan struct{}), //todo make these two buffered?
		newBlockAdded:   make(chan struct{}),
		havePeers:       make(chan bool),
	}

	log.Printf("New Client RPC server created")

	return s, nil
}
