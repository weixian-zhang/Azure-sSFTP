  package main

 
//  //https://github.com/Azure/azure-storage-file-go/blob/master/azfile/zt_examples_test.go

// import (
// 	"github.com/Azure/azure-storage-file-go/azfile"
// 	"net/url"
// 	"fmt"
// 	"context"
// )

// type AzFile struct{
// 	config Config
// 	azfileSvcUrl azfile.ServiceURL
// }

// func NewAzFileClient(config Config) (AzFile) {
// 	cred, err := azfile.NewSharedKeyCredential(config.azStorageName, config.azStorageKey)
// 	logclient.ErrIf(err)

// 	p := azfile.NewPipeline(cred, azfile.PipelineOptions{})

// 	// From the Azure portal, get your Storage account file service URL endpoint.
// 	// The URL typically looks like this:
// 	url, _ := url.Parse(fmt.Sprintf("https://%s.file.core.windows.net", config.azStorageName))

// 	// Create an ServiceURL object that wraps the service URL and a request pipeline.
// 	azfileSvcUrl := azfile.NewServiceURL(*url, p)
	
// 	return AzFile{
// 		config: config,
// 		azfileSvcUrl: azfileSvcUrl,
// 	}
// }

// func (azf AzFile) createFileShares() (error) {

// 	err := azf.createStagingFileShare()
// 	if logclient.ErrIf(err) {
// 		return err
// 	}
	
// 	cerr := azf.createCleanFileShare()
// 	if logclient.ErrIf(cerr) {
// 		return cerr
// 	}

// 	qerr := azf.createQuarantineFileShare()
// 	if logclient.ErrIf(qerr) {
// 		return qerr
// 	}

// 	eerr := azf.createErrorFileShare()
// 	if logclient.ErrIf(eerr) {
// 		return eerr
// 	}

// 	if azf.config.logPath != "" {
// 		lerr := azf.createLogFileShare()
// 		if logclient.ErrIf(lerr) {
// 			return lerr
// 		}
// 	}
// 	return nil
// }

// func (azf AzFile) createStagingFileShare() (error) {

// 	ctx := context.Background()

// 	shareUrl := azf.azfileSvcUrl.NewShareURL(azf.config.stagingFileShareName)
	
// 	_, err := shareUrl.Create(ctx, azfile.Metadata{}, 0)

// 	if err != nil && err.(azfile.StorageError) != nil && err.(azfile.StorageError).ServiceCode() != azfile.ServiceCodeShareAlreadyExists {
// 		return err
// 	} else {
// 		return nil
// 	}
// }

// func (azf AzFile) createCleanFileShare() (error) {

// 	ctx := context.Background()

// 	shareUrl := azf.azfileSvcUrl.NewShareURL(azf.config.cleanFileShareName)
	
// 	_, err := shareUrl.Create(ctx, azfile.Metadata{}, 0)

// 	if err != nil && err.(azfile.StorageError) != nil && err.(azfile.StorageError).ServiceCode() != azfile.ServiceCodeShareAlreadyExists {
// 		return err
// 	} else {
// 		return nil
// 	}
// }

// func (azf AzFile) createQuarantineFileShare() (error) {

// 	ctx := context.Background()

// 	shareUrl := azf.azfileSvcUrl.NewShareURL(azf.config.quarantineFileShareName)
	
// 	_, err := shareUrl.Create(ctx, azfile.Metadata{}, 0)

// 	if err != nil && err.(azfile.StorageError) != nil && err.(azfile.StorageError).ServiceCode() != azfile.ServiceCodeShareAlreadyExists {
// 		return err
// 	} else {
// 		return nil
// 	}
// }

// func (azf AzFile) createErrorFileShare() (error) {

// 	ctx := context.Background()

// 	shareUrl := azf.azfileSvcUrl.NewShareURL(azf.config.errorFileShareName)
	
// 	_, err := shareUrl.Create(ctx, azfile.Metadata{}, 0)

// 	if err != nil && err.(azfile.StorageError) != nil && err.(azfile.StorageError).ServiceCode() != azfile.ServiceCodeShareAlreadyExists {
// 		return err
// 	} else {
// 		return nil
// 	}
// }

// func (azf AzFile) createLogFileShare() (error) {

	
// 	ctx := context.Background()

// 	shareUrl := azf.azfileSvcUrl.NewShareURL(azf.config.logFileShareName)
	
// 	_, err := shareUrl.Create(ctx, azfile.Metadata{}, 0)

// 	if err != nil && err.(azfile.StorageError) != nil && err.(azfile.StorageError).ServiceCode() != azfile.ServiceCodeShareAlreadyExists {
// 		return err
// 	} else {
// 		return nil
// 	}
// }



// func newAzFileClient(storageName string, storageKey string) (azfile.ServiceURL) 	{
// 	cred, err := azfile.NewSharedKeyCredential(storageName, storageKey)
// 	logclient.ErrIf(err)

// 	p := azfile.NewPipeline(cred, azfile.PipelineOptions{})

// 	// From the Azure portal, get your Storage account file service URL endpoint.
// 	// The URL typically looks like this:
// 	url, _ := url.Parse(fmt.Sprintf("https://%s.file.core.windows.net", storageName))

// 	// Create an ServiceURL object that wraps the service URL and a request pipeline.
// 	azfileshare := azfile.NewServiceURL(*url, p)

// 	return azfileshare
// }