$rg = "rgGCCSHOL"
$strgname = "strgssftpintranet111"
$location = "Southeast Asia"
$fsStaging = "ssftp-staging"
$fsClean = "ssftp-clean"
$fsQuarantine = "ssftp-quarantine"
$fsArchive= "ssftp-uploadarchive"
$fsSystem= "ssftp-system"
$fsLog = "ssftp-log"

$storageAcct = New-AzStorageAccount `
    -ResourceGroupName $rg `
    -Name $strgname `
    -Location $location `
    -Kind StorageV2 `
    -SkuName Standard_ZRS `
    -EnableLargeFileShare

$storageContext = (Get-AzStorageAccount -Name $strgname -ResourceGroupName $rg).Context

New-AzStorageShare -Context $storageContext -Name $fsStaging

New-AzStorageShare -Context $storageContext -Name $fsClean

New-AzStorageShare -Context $storageContext -Name $fsQuarantine

New-AzStorageShare -Context $storageContext -Name $fsLog

New-AzStorageShare -Context $storageContext -Name $fsArchive

New-AzStorageShare -Context $storageContext -Name $fsSystem
New-AzStorageDirectory -ShareName $fsSystem -Path "sftpclient" -Context $storageContext
New-AzStorageDirectory -ShareName $fsSystem -Path "sftpclient/downloaders"  -Context $storageContext
New-AzStorageDirectory -ShareName $fsSystem -Path "sftpclient/uploaders"  -Context $storageContext