
package main

import (
	"os"
	"github.com/dutchcoders/go-clamd"
	"fmt"
	"bufio"
)

const clamdAddr string = "tcp://localhost:%s"
const clamdPort string = "3310"

type ScanStatus int32
const (
	OK ScanStatus = iota
	Virus ScanStatus = iota
	Error ScanStatus = iota
)

type ClamAv struct{
	clamClient *clamd.Clamd
	scanEvent chan ClamAvScanResult
	clamdError chan ClamdError
}

//happens when clamd container is terminated or connenction can't be established
type ClamdError struct {
	file string
}

type ClamAvScanResult struct {
	filePath string
	fileName string
	Message string
	Size int
	Status ScanStatus
	VirusFound bool
}

func NewClamAvClient() (ClamAv)  {
	clamdSocketAddr :=  fmt.Sprintf(clamdAddr, clamdPort)
	clamClient := clamd.NewClamd(clamdSocketAddr)

	// err := clamClient.Ping()
	// logclient.ErrIf(err)

	scanEvent := make(chan ClamAvScanResult)
	return ClamAv{
		clamClient: clamClient,
		scanEvent: scanEvent,
		clamdError: make(chan ClamdError),
	}
}

func (cav ClamAv) PingClamd() (bool, error) {

	err := cav.clamClient.Ping()

	if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (cav ClamAv) ScanFile(filePath string) ()  {

	//TODO: scan file
	logclient.Infof("Virus scanning file: %s", filePath)

	file, err := os.Open(filePath)
	if logclient.ErrIf(err) {
		status, vf := convertClamdStatusToLocalEnum("ERROR")
		cav.scanEvent <- ClamAvScanResult{
			filePath: filePath,
			fileName: "",
			Message: "error reading file",
			Size: 0,
			Status: status,
			VirusFound: vf,
		}
		return
	}

	fileinfo, ferr := file.Stat()
	logclient.ErrIf(ferr)

	resp, err :=  cav.clamClient.ScanStream(bufio.NewReader(file), nil)
	if err != nil {
		status, vf := convertClamdStatusToLocalEnum("ERROR")
		cav.scanEvent <- ClamAvScanResult{
			filePath: filePath,
			fileName: fileinfo.Name(),
			Message: err.Error(),
			Size: 0,
			Status: status,
			VirusFound: vf,
		}

		cav.clamdError <- ClamdError{
			file: filePath,
		}

		return
	}

	result := <- resp

	status, vf := convertClamdStatusToLocalEnum(result.Status)
	scanResult := ClamAvScanResult{
		filePath: filePath,
		fileName: fileinfo.Name(),
		Message: result.Raw,
		Size: int(fileinfo.Size()),
		Status: status,
		VirusFound: vf,
	}
	
	logclient.InfoStruct(scanResult)
	
	fcerr := file.Close()

	if fcerr != nil {
		logclient.ErrIf(fcerr)
	} else {
		logclient.Infof("Virus scan completed for %s, file stream closed", scanResult.filePath)

		cav.scanEvent <- scanResult
	}
}

//convertClamdStatusToLocalEnum also returns virusFound = true/false
func convertClamdStatusToLocalEnum(status string) (ScanStatus, bool) {
	if status == "OK" {
		return OK, false
	} else if status == "FOUND"{
		return Virus, true
	} else if status == "ERROR"{
		return Error, false
	} else {
		return Error, false
	}
}
