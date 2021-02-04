package main

import (
	//"github.com/baruwa-enterprise/clamd"

	//"github.com/mirtchovski/clamav"

	"github.com/dutchcoders/go-clamd"
	"fmt"
	
	"path/filepath"
)

const clamdAddr string = "tcp://localhost:%s"
const clamdPort string = "3310"
var clamClient *clamd.Clamd

type ClamAv struct{}

type ClamAvScanResult struct {
	Path string
	Message string
	Size int
	Status string
}

func (cav ClamAv) NewClamAvClient() ClamAv  {
	clamdSocketAddr :=  fmt.Sprintf(clamdAddr, clamdPort)
	clamClient = clamd.NewClamd(clamdSocketAddr)

	err := clamClient.Ping()
	logclient.ErrIf(err)

	return ClamAv{}
}

func (cav ClamAv) ScanFile(filePath string, sc chan<- ClamAvScanResult) ()  {

	path := filepath.FromSlash(filePath)
	resp, err :=  clamClient.ScanFile(path)
	logclient.ErrIf(err)

	result := <- resp

	sc <- ClamAvScanResult{
		Path: result.Path,
		Message: result.Raw,
		Size: result.Size,
		Status: result.Status,
	}

	
}