package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/radovskyb/watcher"
	"github.com/weixian-zhang/ssftp/user"
)

type ScavengedFileProcessContext struct {
	Name string
	Path string
	Size int64
	User user.User
	TimeCreated string
}

type FileWatcher struct {
	confsvc *ConfigService
	usergov  *user.UserGov
	watcher *watcher.Watcher
	configWatcher *watcher.Watcher

	sftpService *SFTPService

	ScanDone		chan bool
	OverlordProcessedUploadedFile chan string
	BatchedScavengedFilesProcessDone chan bool

	fileCreateChangeEvent chan ScavengedFileProcessContext
	fileMovedEvent chan FileMoveChanContext
}

type FileUploadContext struct {
	Path string
	User user.User
}

type FileMoveChanContext struct {
	IsVirusFound 	 bool
	DestPath 		 string
	ClamavScanResult ClamAvScanResult
}

func NewFileWatcher(sftpService *SFTPService, confsvc *ConfigService, usrgov  *user.UserGov, scanDone chan bool) (FileWatcher) { 

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
		sftpService: sftpService,
		fileCreateChangeEvent:  make(chan ScavengedFileProcessContext),
		fileMovedEvent: make(chan FileMoveChanContext),
		ScanDone: scanDone,
		OverlordProcessedUploadedFile: make(chan string),
		BatchedScavengedFilesProcessDone: make(chan bool),
	}
}

func (fw *FileWatcher) ScavengeUploadedFiles() {

	for {

		logclient.Infof("FileWatcher scavenging directory %s", fw.confsvc.config.StagingPath)

		var files []FileUploadContext

		err := filepath.Walk(fw.confsvc.config.StagingPath, func(path string, info os.FileInfo, err error) error {

			if !info.IsDir() {
				files = append(files, FileUploadContext{
					Path: path,
				})
			}

			return nil
		})

		if logclient.ErrIffmsg("FileWatcher error occured while walking staging dir %s", err, fw.confsvc.config.StagingPath) {
			continue
		}

		if len(files) > 0 {

			logclient.Infof("FileWatcher - detects %d files, batching uploaded files for processing", len(files))

			closedFiles, ok := fw.checkScavengedFilesUploadState(files)

			if !ok {
				logclient.Info("FileWatcher - detected open files, waiting for files to complete uploading")
				time.Sleep(500 * time.Millisecond)
				continue
			}

			logclient.Info("FileWatcher - Upload state check completed")

			go fw.notifyOverlordProcessScavengedFiles(closedFiles)

			//<- fw.BatchedScavengedFilesProcessDone

		}

		logclient.Info("FileWatcher - no file uploaded")
		time.Sleep(3 * time.Second)
	}
}

func (fw *FileWatcher) checkScavengedFilesUploadState(files []FileUploadContext) ([]FileUploadContext, bool) {

	logclient.Info("Checking for files in upload state from SFTP clients")

	closeds := make([]FileUploadContext, 0)

	opens := fw.getAllOpenFilesFromAllServers()

	if len(opens) == 0 {
		logclient.Info("No SFTP open file found")
		return files, true
	}

	for _, o := range opens {
		
		for _, f := range files {

			if o.Path != f.Path {
				closeds = append(closeds, f)
				logclient.Infof("File: %s upload completed by client %s", o.Path, o.User.Name)
			} else {
				logclient.Infof("File %s is in upload state from client %s", o.Path, o.User.Name)
			}
		}
	}
	
	if len(closeds) > 0 {
		return closeds, true
	} else {
		return closeds, false
	}

	
}

func (fw *FileWatcher) getAllOpenFilesFromAllServers() ([]FileUploadContext) {

	var opens []FileUploadContext

	for _, v := range fw.sftpService.servers {
		for _, f := range v.OpenFiles {	
			
			opens = append(opens, FileUploadContext{
				Path: f.Name(),
				User: v.User,
			})
		}
	}

	return opens
}

func (fw *FileWatcher) notifyOverlordProcessScavengedFiles(files []FileUploadContext) {

	for _, f := range files {

		logclient.Infof("FileWatcher notifies Overlord to pick up file %s", f.Path)

		fileOnWatch := ScavengedFileProcessContext{
			Path: filepath.FromSlash(f.Path),
			Name:f.Path,
			User: f.User,
			TimeCreated: (time.Now()).Format(time.ANSIC),
		}

		logclient.Infof("FileWatcher blocks scavenging for Overlord to process file %s", f.Path)
	
		fw.fileCreateChangeEvent <-fileOnWatch //notifies overlord to scan file

		logclient.Infof("FileWatcher unblocks for file %s", f.Path)

		//<- fw.OverlordProcessedUploadedFile
	}

	//logclient.Infof("FileWatcher completed processing for batch uploaded files: %s", ToJsonString(files))

	//fw.BatchedScavengedFilesProcessDone <- true
}


//IsFileStillUploading is blocking until file completes upload
// func (fw *FileWatcher) blocksCheckFileUploadComplete(file string) {

// 	logclient.Infof("FileWatcher checking if file %s still uploading", file)

// 	f, err :=  os.Open(file)

// 	if err != nil {
// 		logclient.ErrIffmsg("Error opening file %s", err, file)
// 		return
// 	}

// 	defer f.Close()

// 	logclient.Infof("FileWatcher starts checking file uploading for %s", file)

// 	var totalSize int64

// 	for {

// 		i, serr := os.Stat(file)
		
// 		// // if time.Now().After(i.ModTime()) {
// 		// // 	break
// 		// // }

// 		// b, rerr := 
// 		if serr != nil {
// 			logclient.ErrIffmsg("Filewatcher - checks uploading Stat error for file %s", serr, file)
// 			continue
// 		}

// 		logclient.Infof("modtime: %s, Stat: %d", i.ModTime().String(), i.Size())
// 		logclient.Infof("now: %s, totalSize: %d", time.Now().String(), totalSize)

// 		if i.Size() == totalSize {
// 			break
// 		} else {
// 			totalSize = i.Size()
// 			time.Sleep(600 * time.Millisecond)
// 		}

// 		//https://stackoverflow.com/questions/41208359/how-to-test-eof-on-io-reader-in-go

// 		// dataBuf := make([]byte, 1024)

// 		// n, ferr :=  f.Read(dataBuf)

// 		// if ferr != nil {
// 		// 	if ferr == io.EOF {
// 		// 		logclient.Infof("FileWatcher - file %s upload completed", file)
// 		// 		break
// 		// 	} else {
// 		// 		logclient.ErrIffmsg("FileWatcher - Read error for file %s", ferr, file)
// 		// 		continue
// 		// 	}
// 		// }

// 		// logclient.Infof("length of file read: %d", n)

// 		// if n > 0 {
			
// 		// 	logclient.Infof("FileWatcher - file %s upload in progess, %d Mb", file, fw.fileSizeMb(file))

// 		// 	time.Sleep(200 * time.Millisecond)
			
// 		// 	continue
// 		// }

// 		// logclient.Infof("FileWatcher - file %s upload completed", file)
// 		// break
// 	}
// }

// func (fw *FileWatcher) fileSizeMb(file string) (newSize int) {
// 	info, _ := os.Stat(file)

// 	mb := info.Size() / (1024 * 1024)

// 	newSize = int(mb)

// 	return newSize
// }

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


// func ByByteOne(rd io.Reader, function func(byte)) error {

// 	logclient.Info("ByByteOne starts detecting file uploading")

// 	bufferedRD := bufio.NewReader(rd)

// 	for {
		

// 		fileByte, err := bufferedRD.ReadByte()
// 		if err == io.EOF {

// 			logclient.Info("ByByteOne detected EOF for file")

// 			break
// 		} else if err != nil {
// 			return err
// 		}

// 		function(fileByte)
// 	}

// 	logclient.Info("ByByteOne detected EOF")

// 	return nil
// }



