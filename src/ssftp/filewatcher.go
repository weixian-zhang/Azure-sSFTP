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
	TimeCreated string
}

type FileWatcher struct {
	confsvc *ConfigService
	usergov  *user.UserGov
	watcher *watcher.Watcher
	configWatcher *watcher.Watcher

	sftpService *SFTPService

	ScanDone		chan bool
	//OverlordProcessedUploadedFile chan string
	BatchedScavengedFilesProcessDone chan bool

	fileCreateChangeEvent chan ScavengedFileProcessContext
	fileMovedEvent chan FileMoveChanContext
}

type FileUploadContext struct {
	Path string
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
		//OverlordProcessedUploadedFile: make(chan string),
		BatchedScavengedFilesProcessDone: make(chan bool),
	}
}

//ScavengeUploadedFiles might scavenge files that are still uploading, especially large files which take time.
//Addition checks in-place to detect and leave uploading files alone
func (fw *FileWatcher) ScavengeUploadedFiles() {

	for {

		logclient.Infof("FileWatcher - scavenging new files in directory %s", fw.confsvc.config.StagingPath)

		var files []FileUploadContext

		err := filepath.Walk(fw.confsvc.config.StagingPath, func(path string, info os.FileInfo, err error) error {

			if !info.IsDir() {
				files = append(files, FileUploadContext{
					Path: path,
				})
			}

			return nil
		})

		if logclient.ErrIffmsg("FileWatcher error occured while scavenging %s", err, fw.confsvc.config.StagingPath) {
			continue
		}

		if len(files) > 0 {

			logclient.Infof("FileWatcher - detects %d files", len(files))

			closedFiles, ok := fw.checkScavengedFilesUploadState(files)

			if !ok {
				logclient.Info("FileWatcher - detected open files, waiting for files to complete upload")
				time.Sleep(500 * time.Millisecond)
				continue
			}

			logclient.Info("FileWatcher - Upload state check completed")

			go fw.notifyOverlordProcessScavengedFiles(closedFiles)

			<- fw.BatchedScavengedFilesProcessDone
		}

		time.Sleep(3 * time.Second)
	}
}

func (fw *FileWatcher) checkScavengedFilesUploadState(files []FileUploadContext) ([]FileUploadContext, bool) {
	logclient.Info("FileWatcher - Checking file upload state")

	closeds := make([]FileUploadContext, 0)

	opens := fw.getAllOpenFilesFromAllServers()

	if len(opens) == 0 {
		logclient.Info("FileWatcher - No on-going file upload detected")
		return files, true
	}

	for _, f := range files {

		if fw.isScavengedFileInOpenFileList(f.Path, opens) {
			logclient.Infof("FileWatcher - File %s is in upload state, current size %dMB", f.Path, fw.fileSizeMb(f.Path))
			time.Sleep(300 * time.Millisecond)
			continue
		} else {
			closeds = append(closeds, f)
		}
	}
	
	if len(closeds) > 0 {
		return closeds, true
	} else {
		return closeds, false
	}
}

func (fw *FileWatcher) isScavengedFileInOpenFileList(sfile string, opens []FileUploadContext) bool {
	for _, a := range opens {
        if a.Path == sfile {
            return true
        }
    }
    return false
}

func (fw *FileWatcher) getAllOpenFilesFromAllServers() ([]FileUploadContext) {

	var opens []FileUploadContext

	for _, v := range fw.sftpService.servers {
		for _, f := range v.OpenFiles {	

			if isDir(f.Name()) {
				continue
			}
			
			opens = append(opens, FileUploadContext{
				Path: f.Name(),
			})
		}
	}

	return opens
}

func (fw *FileWatcher) notifyOverlordProcessScavengedFiles(files []FileUploadContext) {

	for _, f := range files {

		logclient.Infof("FileWatcher notifies Overlord to pick up file %s", f.Path)

		//notifies overlord to scan file
		sFile := ScavengedFileProcessContext{
			Path: filepath.FromSlash(f.Path),
			Name:f.Path,
			TimeCreated: (time.Now()).Format(time.ANSIC),
		}

		fw.fileCreateChangeEvent <- sFile

		logclient.Infof("FileWatcher blocks for Overlord to scan file %s", f.Path)
	
		 <- fw.ScanDone //wait for scan done

		logclient.Infof("FileWatcher - scan done, unblocks from scanning file %s", f.Path)
	}

	fw.BatchedScavengedFilesProcessDone <- true
}

func (fw *FileWatcher) fileSizeMb(file string) (newSize int) {
	info, err := os.Stat(file)

	if err != nil {
		logclient.ErrIfm("Filewatcher - error while checking fize size on upload", err)
		return 0
	}

	mb := info.Size() / (1024 * 1024)

	newSize = int(mb)

	return newSize
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

						logclient.Infof("FileWatcher - Config file %s change detected", SystemConfigPath)
	
						loaded := fw.confsvc.LoadYamlConfig()

						config := <- loaded

						logclient.Infof("FileWatcher - Config file loaded successfully")

						fw.usergov.SetUsers(config.Users)
					}
				}
		}
	}
}

func (fw *FileWatcher) moveFileToErrorFileshare(file string) {
	errorPath := fw.confsvc.config.ErrorPath

	err := fw.moveFileBetweenDrives(file, errorPath)
	logclient.ErrIfm("FileWatcher - Error moving file to Error fileshare", err)
}

//moveFileByStatus returns destination path where file is moved. Either /mnt/ssftp/clean|quaratine|error
func (fw *FileWatcher) moveFileByStatus(scanR ClamAvScanResult) (string) {

	logclient.Infof("FileWatcher - starts moving file %s by scan status", scanR.filePath)

	//replace "staging" folder path with new Clean and Quarantine path so when file is moved to either
	//clean/quarantine, the sub-folder structure remains the same as staging.
	//e.g: Staging:/mnt/ssftp/'staging'/userB/sub = Clean:/mnt/ssftp/'clean'/userB/sub or Quarantine:/mnt/ssftp/'quarantine'/userB/sub
	cleanPath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.CleanPath, -1)
	quarantinePath := strings.Replace(scanR.filePath, fw.confsvc.config.StagingPath, fw.confsvc.config.QuarantinePath, -1)

	destPath := cleanPath
	
	if !scanR.VirusFound {

		err := fw.moveFileBetweenDrives(scanR.filePath,cleanPath)
		logclient.ErrIfm("FileWatcher - Error moving file between drives when virus is not found", err)

		logclient.Infof("FileWatcher - Moving clean file %q to %q", scanR.filePath, cleanPath)

	} else {

		destPath = quarantinePath

		err := fw.moveFileBetweenDrives(scanR.filePath, quarantinePath)
		logclient.ErrIfm("FileWatcher - Error moving file between drives when virus is found", err)
		
		logclient.Infof("FileWatcher - Virus found in file %q, moving to quarantine %q", scanR.filePath, quarantinePath)
	}

	logclient.Infof("FileWatcher - move file completed, new destication: %s", destPath)

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



