package main

import (
	"os"
	"github.com/dutchcoders/go-clamd"
	"fmt"
	"encoding/json"
	"bufio"
)

const clamdAddr string = "tcp://localhost:%s"
const clamdPort string = "3310"
var clamClient *clamd.Clamd

type ScanStatus int32
const (
	OK ScanStatus = iota
	Virus ScanStatus = iota
	Error ScanStatus = iota
)

type ClamAv struct{
	scanEvent chan ClamAvScanResult
}

type ClamAvScanResult struct {
	filePath string
	fileName string
	Message string
	Size int
	Status ScanStatus
}

func NewClamAvClient() (ClamAv, error)  {
	clamdSocketAddr :=  fmt.Sprintf(clamdAddr, clamdPort)
	clamClient = clamd.NewClamd(clamdSocketAddr)

	err := clamClient.Ping()
	logclient.ErrIf(err)

	scanEvent := make(chan ClamAvScanResult)
	return ClamAv{
		scanEvent: scanEvent,
	}, err
}

func (cav ClamAv) ScanFile(filePath string) ()  {

	//TODO: scan file
	logclient.Infof("scanning file: %s", filePath)

	file, err := os.Open(filePath)
	if logclient.ErrIf(err) {
		cav.scanEvent <- ClamAvScanResult{
			filePath: filePath,
			fileName: "",
			Message: "error reading file",
			Size: 0,
			Status: convertClamdStatusToLocalEnum("ERROR"),
		}
		return
	}
	defer file.Close()

	fileinfo, ferr := file.Stat()
	logclient.ErrIf(ferr)

	resp, err :=  clamClient.ScanStream(bufio.NewReader(file), nil)
	if logclient.ErrIf(err) {
		cav.scanEvent <- ClamAvScanResult{
			filePath: filePath,
			fileName: fileinfo.Name(),
			Message: "clamav encountered an error scanning file",
			Size: 0,
			Status: convertClamdStatusToLocalEnum("ERROR"),
		}
		return
	}

	result := <- resp

	scanResult := ClamAvScanResult{
		filePath: filePath,
		fileName: fileinfo.Name(),
		Message: result.Raw,
		Size: int(fileinfo.Size()),
		Status: convertClamdStatusToLocalEnum(result.Status),
	}
	
	logclient.InfoStruct(scanResult)
	
	cav.scanEvent <- scanResult
}

func convertClamdStatusToLocalEnum(status string) (ScanStatus) {
	if status == "OK" {
		return OK
	} else if status == "FOUND"{
		return Virus
	} else if status == "ERROR"{
		return Error
	} else {
		return Error 
	}
}
