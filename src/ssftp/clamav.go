package main

import (
	//"bufio"
	"fmt"
	"github.com/dutchcoders/go-clamd"
)

//self setup Clamav
//https://github.com/Flowman/docker-clamav

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
	err string
}

type ClamAvScanResult struct {
	filePath string
	Message string
	Status ScanStatus
	VirusFound bool
	Error bool
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

func (cav *ClamAv) PingClamd() (bool, error) {

	err := cav.clamClient.Ping()

	if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (cav *ClamAv) ScanFile(filePath string, enableVirusScan bool) ()  {

	if !enableVirusScan {
		s, _ := cav.convertClamdStatusToLocalEnum("OK")
		scanResult := ClamAvScanResult{
			filePath: filePath,
			Message: "ClamAV scan disabled in config",
			Status: s,
			VirusFound: false,
			Error: false,
		}
		
		logclient.Infof("*ClamAV - file scan disabled in config for %s", filePath)
	
		cav.scanEvent <- scanResult	

		return
	}

	
	//TODO: scan file
	logclient.Infof("ClamAV - start scanning file: %s", filePath)

	resp, err :=  cav.clamClient.ScanFile(filePath) //)ScanStream(bufio.NewReader(file), nil)
	if err != nil {
		status, vf := cav.convertClamdStatusToLocalEnum("ERROR")
		cav.scanEvent <- ClamAvScanResult{
			filePath: filePath,
			Message: err.Error(),
			Status: status,
			VirusFound: vf,
			Error: true ,
		}
		return
	}

	result := <- resp

	status, vf := cav.convertClamdStatusToLocalEnum(result.Status)
	scanResult := ClamAvScanResult{
		filePath: filePath,
		Message: result.Raw,
		Status: status,
		VirusFound: vf,
		Error: false,
	}
	
	logclient.Infof("Virus scan completed for %s, file stream closed", scanResult.filePath)

	cav.scanEvent <- scanResult		
}

//convertClamdStatusToLocalEnum also returns virusFound = true/false
func (cav *ClamAv) convertClamdStatusToLocalEnum(status string) (ScanStatus, bool) {
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
