
package main

import (
	//"fmt"
	"time"
	"os"
	"io"
	"github.com/radovskyb/watcher"
	"path/filepath"
	"github.com/weixian-zhang/ssftp/user"
	"strings"
	"math"
	"bufio"
)

type File struct {
	Name string
	Path string
	Size int64
	TimeCreated string
}

type FileWatcher struct {
	confsvc *ConfigService
	usergov  *user.UserGov
	watcher *watcher.Watcher
	configWatcher *watcher.Watcher

	ScanDone		chan bool
	OverlordProcessedUploadedFile chan string
	BatchedUploadedFilesProcessDone chan bool

	fileCreateChangeEvent chan File
	fileMovedEvent chan FileMoveChanContext
}

type FileUploadContext struct {
	Path string
	Info os.FileInfo
}

type FileMoveChanContext struct {
	IsVirusFound 	 bool
	DestPath 		 string
	ClamavScanResult ClamAvScanResult
}

func NewFileWatcher(confsvc *ConfigService, usrgov  *user.UserGov, scanDone chan bool) (FileWatcher) { 

	w := watcher.New()
	w.FilterOps(watcher.Create, watcher.Write, watcher.Rename)

	aerr := w.AddRecursive(confsvc.config.StagingPath)
	logclient.ErrIf(aerr)
	
	cw := watcher.New()
	cw.FilterOps(watcher.Write)
	conferr := cw.AddRecursive(confsvc.getYamlConfgPath())
	logclient.ErrIf(conferr)

	return FileWatcher{
		watcher: w,
		configWatcher: cw,
		confsvc: confsvc,
		usergov: usrgov,
		fileCreateChangeEvent:  make(chan File),
		fileMovedEvent: make(chan FileMoveChanContext),
		ScanDone: scanDone,
		OverlordProcessedUploadedFile: make(chan string),
		BatchedUploadedFilesProcessDone: make(chan bool),
	}
}

func (fw *FileWatcher) StartPickupUploadedFiles() {

	for {

		logclient.Infof("FileWatcher walks directory %s to pick up uploaded files", fw.confsvc.config.StagingPath)

		var files []FileUploadContext

		err := filepath.Walk(fw.confsvc.config.StagingPath, func(path string, info os.FileInfo, err error) error {

			if !info.IsDir() {
				files = append(files, FileUploadContext{
					Path: path,
					Info: info,
				})
			}

			return nil
		})

		if logclient.ErrIffmsg("FileWatcher error occured while walking staging dir %s", err, fw.confsvc.config.StagingPath) {
			continue
		}

		if len(files) > 0 {

			logclient.Infof("FileWatcher detects %d files, batching uploaded files for processing: %s", ToJsonString(files))

			go fw.notifyOverlordOnFileUploaded(files)

			<- fw.BatchedUploadedFilesProcessDone

		} else {
			logclient.Info("FileWatcher detects 0 file uploaded")
			time.Sleep(3 * time.Second)
		}
	}
}

func (fw *FileWatcher) notifyOverlordOnFileUploaded(files []FileUploadContext) {

	for _, f := range files {

		yes, err := fw.IsFileStillUploading(f.Path)
		logclient.ErrIffmsg("Error while checking if file %s is still uploading", err, f.Path)

		if !yes {
			continue
		}

		logclient.Infof("FileWatcher notifies Overlord on pick up file %s", f.Path)

		fileOnWatch := File{
			Path: filepath.FromSlash(f.Path),
			Name:f.Info.Name(), 
			Size: f.Info.Size(),
			TimeCreated: (time.Now()).Format(time.ANSIC),
		}

		logclient.Infof("FileWatcher blocks for Overlord to process file %s", f.Path)
	
		fw.fileCreateChangeEvent <-fileOnWatch //notifies overlord to scan file

		logclient.Infof("FileWatcher unblocks for file %s", f.Path)

		<- fw.OverlordProcessedUploadedFile
	}

	logclient.Infof("FileWatcher completed processing for batch uploaded files: %s", ToJsonString(files))

	fw.BatchedUploadedFilesProcessDone <- true
}

func ByByteOne(rd io.Reader, function func(byte)) error {

	logclient.Info("ByByteOne starts detecting file uploading")

	bufferedRD := bufio.NewReader(rd)

	for {
		

		fileByte, err := bufferedRD.ReadByte()
		if err == io.EOF {

			logclient.Info("ByByteOne detected EOF for file")

			break
		} else if err != nil {
			return err
		}

		function(fileByte)
	}

	logclient.Info("ByByteOne detected EOF")

	return nil
}

func (fw *FileWatcher) IsFileStillUploading(file string) (bool, error) {

	// pr, pw := io.Pipe()

	logclient.Infof("FileWatcher checking if file %s still uploading", file)

	f, err :=  os.Open(file)

	if err != nil {
		logclient.ErrIffmsg("Error opening file %s", err, file)
		return false, err
	}

	defer f.Close()

	// bberr := ByByteOne(f, func(b byte) {
	// 	//logclient.Info("ByByteOne func print byte still reading")
	// })

	//logclient.ErrIfm("ByByteOne throws error", bberr)

	logclient.Infof("FileWatcher starts checking file uploading for %s", file)

	for {

		//https://stackoverflow.com/questions/41208359/how-to-test-eof-on-io-reader-in-go

		dataBuf := make([]byte, 10000)

		_, ferr :=  f.Read(dataBuf)

		if ferr != nil {
			if ferr == io.EOF {
				logclient.Infof("File %s upload completes", file)
				break
			} else {
				info, err := os.Stat(file)
				logclient.ErrIffmsg("File is still uploading %s, %fMB", err, file, fw.roundToMB(float64(info.Size()), .5, 2))

				continue
			}
		}

		if len(dataBuf) > 0 {
			logclient.Infof("File %s still uploading", file)
			continue
		} else {
			logclient.Infof("File %s completed upload", file)
			break
		}

	// 	} else {
	// 		logclient.Infof("File %s upload completes", file)
	// 		break
	// 	}
	// // }
	}

	return true, nil
}

func (fw *FileWatcher) roundToMB(val float64, roundOn float64, places int ) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func (fw *FileWatcher) startWatchConfigFileChange() {

	go fw.registerConfigFileChangeEvent()

	serr := fw.configWatcher.Start(time.Millisecond * 300)
	logclient.ErrIf(serr)
}

func (fw *FileWatcher) registerConfigFileChangeEvent() {
	for {
		select {
			case err := <- fw.configWatcher.Error:
				logclient.ErrIf(err)

			case event := <- fw.configWatcher.Event:

				if event.IsDir() {
					continue
				}

				if event.Op == watcher.Create || event.Op == watcher.Write  {

					if SystemConfigFileName ==  event.Name() {

						logclient.Infof("Config file %s change detected", SystemConfigPath)
	
						loaded := fw.confsvc.LoadYamlConfig()

						config := <- loaded

						logclient.Infof("Config file loaded successfully")

						fw.usergov.SetUsers(config.Users)
					}
				}
		}
	}
}

//moveFileByStatus returns destination path where file is moved. Either /mnt/ssftp/clean|quaratine|error
func (fw *FileWatcher) moveFileByStatus(scanR ClamAvScanResult) (string) {

	logclient.Infof("FileWatcher starts moving file %s by scan status", scanR.fileName)

	//replace "staging" folder path with new Clean and Quarantine path so when file is moved to either
	//clean/quarantine, the sub-folder structure remains the same as staging.
	//e.g: Staging:/mnt/ssftp/'staging'/userB/sub = Clean:/mnt/ssftp/'clean'/userB/sub or Quarantine:/mnt/ssftp/'quarantine'/userB/sub
	cleanPath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.CleanPath, -1)
	quarantinePath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.QuarantinePath, -1)

	destPath := cleanPath
	
	if !scanR.VirusFound {

		err := fw.moveFileBetweenDrives(scanR.filePath,cleanPath)
		logclient.ErrIfm("Error moving file between drives when virus is not found", err)

		logclient.Infof("Moving clean file %q to %q", scanR.fileName, cleanPath)

	} else {

		destPath = quarantinePath

		err := fw.moveFileBetweenDrives(scanR.filePath, quarantinePath)
		logclient.ErrIfm("Error moving file between drives when virus is found", err)
		
		logclient.Infof("Virus found in file %q, moving to quarantine %q", scanR.fileName, quarantinePath)
	}

	logclient.Infof("FileWatcher move file completed, new destication: %s", destPath)

	fw.OverlordProcessedUploadedFile <- destPath //unblocks filepath.Walk to start pickup n

	return destPath
}

//moveFileBetweenDrives also creates subfolders following staging/../.. if any
func (fw *FileWatcher) moveFileBetweenDrives(srcPath string, destPath string) (error) {

	//file has moved by other goroutine
	if !isFileExist(srcPath) {
		return nil
	}

	srcFile, err := os.Open(srcPath)
    if os.IsNotExist(err) {
		return nil
	}

	//creates all subfolders following staging/.../... if any
	destDirPathonly := filepath.Dir(destPath)
	if err := os.MkdirAll(destDirPathonly, os.ModePerm); os.IsExist(err) {
		logclient.Infof("Path exist %s", destDirPathonly)
	}

    destFile, err := os.Create(destPath)
    if logclient.ErrIf(err) {
		srcFile.Close()
		return err
	}

    defer destFile.Close()

    _, err = io.Copy(destFile, srcFile)
    srcFile.Close()
    if err != nil {
        logclient.ErrIf(err)
		return err
    }

    // The copy was successful, so now delete the original file
    err = os.Remove(srcPath)
    if logclient.ErrIf(err) {
		return err
	}

	return nil
}



