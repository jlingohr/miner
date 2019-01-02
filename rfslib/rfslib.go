/*

This package specifies the application's interface to the distributed
records system (RFS) to be used in project 1 of UBC CS 416 2018W1.

You are not allowed to change this API, but you do have to implement
it.

*/

package rfslib

import (
	"fmt"
	"net"
	"net/rpc"
)

// A Record is the unit of file access (reading/appending) in RFS.
type Record [512]byte

type AppendRecordInfo struct {
	Data  Record
	Fname string
}

type ReadRecordInfo struct {
	RecNum int
	Fname  string
}

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains minerAddr
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("RFS: Disconnected from the bclib [%s]", string(e))
}

// Contains filename. The *only* constraint on filenames in RFS is
// that must be at most 64 bytes long.
type BadFilenameError string

func (e BadFilenameError) Error() string {
	return fmt.Sprintf("RFS: Filename [%s] has the wrong length", string(e))
}

// Contains filename
type FileDoesNotExistError string

func (e FileDoesNotExistError) Error() string {
	return fmt.Sprintf("RFS: Cannot open file [%s] in D mode as it does not exist locally", string(e))
}

// Contains filename
type FileExistsError string

func (e FileExistsError) Error() string {
	return fmt.Sprintf("RFS: Cannot create file with filename [%s] as it already exists", string(e))
}

// Contains filename
type FileMaxLenReachedError string

func (e FileMaxLenReachedError) Error() string {
	return fmt.Sprintf("RFS: File [%s] has reached its maximum length", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Represents a connection to the RFS system.
type RFS interface {
	// Creates a new empty RFS file with name fname.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileExistsError
	// - BadFilenameError
	CreateFile(fname string) (err error)

	// Returns a slice of strings containing filenames of all the
	// existing files in RFS.
	//
	// Can return the following errors:
	// - DisconnectedError
	ListFiles() (fnames []string, err error)

	// Returns the total number of records in a file with filename
	// fname.
	//
	// Can return the following errors:
	// - DisconnectedError
	// - FileDoesNotExistError
	TotalRecs(fname string) (numRecs uint16, err error)
	//
	//// Reads a record from file fname at position recordNum into
	//// memory pointed to by record. Returns a non-nil error if the
	//// read was unsuccessful. If a record at this index does not yet
	//// exist, this call must block until the record at this index
	//// exists, and then return the record.
	////
	//// Can return the following errors:
	//// - DisconnectedError
	//// - FileDoesNotExistError
	ReadRec(fname string, recordNum uint16, record *Record) (err error)
	//
	//// Appends a new record to a file with name fname with the
	//// contents pointed to by record. Returns the position of the
	//// record that was just appended as recordNum. Returns a non-nil
	//// error if the operation was unsuccessful.
	////
	//// Can return the following errors:
	//// - DisconnectedError
	//// - FileDoesNotExistError
	//// - FileMaxLenReachedError
	AppendRec(fname string, record *Record) (recordNum uint16, err error)
}

// The constructor for a new RFS object instance. Takes the bclib's
// IP:port address string as parameter, and the localAddr which is the
// local IP:port to use to establish the connection to the bclib.
//
// The returned rfs instance is singleton: an application is expected
// to interact with just one rfs at a time.
//
// This call should only succeed if the connection to the bclib
// succeeds. This call can return the following errors:
// - Networking errors related to localAddr or minerAddr
func Initialize(localAddr string, minerAddr string) (rfs RFS, err error) {
	// TODO
	// For now return a DisconnectedError
	_, err = net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, DisconnectedError(localAddr)
	}

	_, err = net.ResolveTCPAddr("tcp", minerAddr)
	if err != nil {
		return nil, DisconnectedError(minerAddr)
	}

	conn, err := rpc.Dial("tcp", minerAddr)
	if err != nil {
		return nil, DisconnectedError(minerAddr)
	}

	client := &RFSClient{
		client: conn,
	}

	return client, nil

}

type RFSClient struct {
	client *rpc.Client
}

func (c *RFSClient) CreateFile(fname string) (err error) {
	// todo check that filename is not too long, err otherwise
	if len([]byte(fname)) > 64 {
		return BadFilenameError(fname)
	}

	var created bool
	err = c.client.Call("Server.CreateFile", fname, &created)

	if err != nil {
		return err
	}

	return nil
}

func (c *RFSClient) ListFiles() (fnames []string, err error) {
	reply := make([]string, 0)
	err = c.client.Call("Server.ListFiles", 1, &reply)

	if err != nil {
		return nil, err
	}

	return reply, err
}

func (c *RFSClient) TotalRecs(fname string) (numRecs uint16, err error) {
	var reply int
	err = c.client.Call("Server.TotalRecs", fname, &reply)
	if err != nil {
		return 0, err
	}

	return uint16(reply), nil
}

func (c *RFSClient) ReadRec(fname string, recordNum uint16, record *Record) (err error) {
	var reply Record

	recordInfo := ReadRecordInfo{
		RecNum: int(recordNum),
		Fname:  fname,
	}

	err = c.client.Call("Server.ReadRecord", recordInfo, &reply)
	if err != nil {
		return err
	}

	*record = reply

	return nil
}

func (c *RFSClient) AppendRec(fname string, record *Record) (recordNum uint16, err error) {
	var reply int

	recordInfo := AppendRecordInfo{
		Data:  *record,
		Fname: fname,
	}

	err = c.client.Call("Server.AppendRecord", recordInfo, &reply)
	if err != nil {
		return 0, err
	}

	return uint16(reply), nil
}
