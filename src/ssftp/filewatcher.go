package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/radovskyb/watcher"
	"github.com/weixian-zhang/ssftp/user"
	"github.com/weixian-zhang/ssftp/xattr"
)

const ExtendedAttriPrefix = "user."

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
	UploadTimeup chan FileUploadContext
	BatchedScavengedFilesProcessDone chan bool

	fileCreateChangeEvent chan ScavengedFileProcessContext
}

type FileUploadContext struct {
	Path string
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
		ScanDone: scanDone,
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
				
				if !fw.isScavengedFileDownloadState(path) {
					files = append(files, FileUploadContext{
						Path: path,
					})
				} else {
					logclient.Infof("Filewatcher - detected file %s in downloading state, skipping file", path)
				}
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

//checkScavengedFileDownloadState checks extended attribute of file set by Downloader
//to determine if download has completed before sending file fo scanning and move
func (fw *FileWatcher) isScavengedFileDownloadState(path string) (bool) {

	data, err := xattr.Get(path, ExtendedAttriPrefix + "isdownloading");
	if err != nil {
		//logclient.ErrIffmsg("FileWatcher - error checking file download state for %s", err, path)
		return false
	}

	if data != nil && string(data) == "false" {
		return false
	} else {
		return true
	}

}

//checkScavengedFilesUploadState checks files' upload state where info is supplied by pkgsftp.server.go
func (fw *FileWatcher) checkScavengedFilesUploadState(scanvengedFiles []FileUploadContext) ([]FileUploadContext, bool) {
	logclient.Info("FileWatcher - Checking file upload state")

	closeds := make([]FileUploadContext, 0)

	opens := fw.getSFTPOpenFiles() //sync open files with tracker slice

	if len(opens) == 0 {
		logclient.Info("FileWatcher - No on-going file upload detected")
		return scanvengedFiles, true
	}

	for _, f := range scanvengedFiles {

		//checking of SFTP.Server.OpenFiles() may at times not accurate due to go.sftp returning positive open file result
		//while in actual fact client already completed uploading
		if fw.isScavengedFileInOpenFileList(f.Path, opens) {

			ofile, isTimeup, durMins, modtimestr, err := fw.isSFTPOpenFileTimeup(f.Path, opens)

			if err != nil {
				logclient.ErrIfm("Filewatcher - error while checking open file time limit reach", err)
				continue
			}

			//addition check on upload file mod time if idling more than time limit, auto time out
			if isTimeup {
				
				ofile.Close()

				logclient.Infof("FileWatcher - File %s with size %dMB still in upload state. Last mod time %s, upload duration %d mins. Reached upload idle limit %d mins, file closed and timing out now", f.Path, fw.fileSizeMb(f.Path), modtimestr, durMins, UploadTimeLimitMin)
				closeds = append(closeds, f)

			} else {
				logclient.Infof("FileWatcher - File %s is in upload state, last mode time %s, upload duration %d/%d mins, size %dMB", f.Path, modtimestr, durMins, UploadTimeLimitMin, fw.fileSizeMb(f.Path))
				time.Sleep(500 * time.Millisecond)
			}
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

func (fw *FileWatcher) isScavengedFileInOpenFileList(sfile string, opens []*os.File) (bool) {
	for _, o := range opens {
        if o.Name() == sfile {
            return true
        }
    }
    return false
}

//isSFTPOpenFileTimeup returns:
//os.File of uploading file
//upload time up,
//upload duration since last mod time in minutes string
//last mod time in string
func (fw *FileWatcher) isSFTPOpenFileTimeup(scavengedFile string, opens []*os.File) (*os.File, bool, int, string, error) {
	for _, o := range opens {
		if scavengedFile == o.Name() {

			modTime, err := fw.fileModTime(o.Name())

			if err != nil {
				logclient.ErrIfm("Filewatcher - error while checking uploading file stats", err)
				return o, false, 0, "unknown", err
			}

			durMin := fw.uploadTimeDurationMins(modTime)

			lastmodtimef := modTime.Format(time.ANSIC)

			if durMin >= UploadTimeLimitMin {
				return o, true, durMin, lastmodtimef, nil
			} else {
				return o, false, durMin, lastmodtimef, nil
			}
		}
	}

	modTime, err := fw.fileModTime(scavengedFile)

	return nil, false, fw.uploadTimeDurationMins(modTime), modTime.Format(time.ANSIC), err
}

func (fw *FileWatcher) getSFTPOpenFiles() ([]*os.File) {

	var opens []*os.File

	for _, v := range fw.sftpService.servers {
		for _, sftpO := range v.OpenFiles {		
			opens = append(opens, sftpO)
		}
	}

	return opens
}

func (fw *FileWatcher) notifyOverlordProcessScavengedFiles(files []FileUploadContext) {

	for _, f := range files {

		logclient.Infof("FileWatcher - notify Overlord to pick up file %s", f.Path)

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

func (fw *FileWatcher) uploadTimeDurationMins(fmodTime time.Time) (int) {

	uploadtimeDur := time.Since(fmodTime)
	
	return int(uploadtimeDur.Minutes())
}

func (fw *FileWatcher) fileModTime(file string) (time.Time, error) {
	info, err := os.Stat(file)

	if err != nil {
		logclient.ErrIfm("Filewatcher - error while checking file mod time on upload", err)
		return time.Now(), err
	}

	return info.ModTime(), nil
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

func (fw *FileWatcher) startWatchConfigFileChange(configChange chan bool) {

	go fw.registerConfigFileChangeEvent(configChange)

	serr := fw.configWatcher.Start(time.Millisecond * 300)
	logclient.ErrIf(serr)
}

func (fw *FileWatcher) registerConfigFileChangeEvent(configChange chan bool) {
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

						fw.confsvc.config = &config

						logclient.Infof("FileWatcher - Config file loaded successfully")

						configChange <- true
					}
				}
		}
	}
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



