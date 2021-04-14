## Azure Scanned SFTP  

* [What is sSFTP](#what-is-ssftp)
* [Features](#features)
* [How Things Work - Directories & Conventions](#how-things-work---directories--conventions)
* [How Things Work - Proposed Deployment Architecture](#how-things-work---proposed-deployment-architecture)
* [Configuring sSFTP](#configuring-ssftp)
* [Deploy sSFTP](#deploy-ssftp)
* [Webhook](#webhook)
* [Networking](#networking) 

### What is sSFTP
Azure sSFTP (Scanned SFTP) is a Go app deployed on Azure Container Instance to provide SFTP server and client services with integrated [ClamAV](https://www.clamav.net/) virus scanning and Azure File as file storage.  
sSFTP consists of 2 containers into a single Container Group namely
* [ClamAV container (by mkodockx)](https://hub.docker.com/r/mkodockx/docker-clamav/) with selfupdating of virus signature and Clamd (daemon) listening to port 3310 for virus scan commands.
* [sSFTP container (by weixian)](https://hub.docker.com/repository/docker/wxzd/ssftp) runs a SFTP server, watches for uploaded files, scans and sort files into appropriate mounted directories to isolate clean and virus-detected files.  

### Features  

* Container-based solution that runs on Azure Container Instance (PaaS), no infrastructure maintainence needed
* sSFTP's runs securely in Virtual Network while Internet traffic to SFTP server is proxied through Azure Firewall or Firewall of your choice
* Built-in Sftp server 
* Built-in Sftp clients to support multiple concurrent download and upload files to and from remote SFTP servers
* ClamAV virus scan on:
  * uploaded files from external Sftp clients
  * files downloaded by sSFTP Downloaders
* Supports certificate and password authentication
* Azure File as the file storage for SFTP server
* Supports [Webhook invocation](#webhook) when virus is detected
* Each Sftp login account is jailed to its configured directory only
* Configurable with a single Yaml file, config changes are recognize instantly with no container restart needed
* In the roadmap
    * Additional logging destinations like Log Analytics Workspace, Azure SQL, Azure Cosmos and more
    * Web portal to configure sSFTP in addition to current Yaml file format. Web Portal will be co-hosted within sSFTP container.

### How Things Work - Directories & Conventions  

<img src="./doc/ssftp-modules-directories.png" width="850" height="600" />  

* sSFTP at it's core provides a built-in Sftp server that supports multiple concurrent Sftp clients to connect and upload files.
  * Uploaded files are by design saved to <b>Staging directory(/mnt/ssftp/staging)</b>
  * FileWatcher picks up files from Staging directory and nested sub-directories and send them for ClamAV scanning
  * FileWatcher moves Virus-free files determined by ClamAV <b>Clean directory(/mnt/ssftp/clean)</b>
  * FileWatcher moves files containing virus to <b>Quarantine directory(/mnt/ssftp/quarantine)</b>  
Above process is performed on each uploaded file.  
  
* The Downloader module are Sftp clients that downloads from remote Sftp server. You can configure multiple Downloaders through [ssftp.yaml](https://github.com/weixian-zhang/Azure-sSFTP/blob/main/deploy/ssftp.yaml) to support concurrent downloads from remote Sftp servers.  
  <b>*Downloaded files are save to Staging directory(/mnt/ssftp/staging) for FileWatcher to scan and sort.</b>
  
* Similarly to Downloaders, Uploaders are Sftp clients that uploads files to remote Sftp servers and supports multiple Uploaders running concurrently.   
  <b>*Uploaders only upload files from Clean directory(/mnt/ssftp/clean), nested directories in Clean directory are supported</b>

* FileWatcher creates the same nested sub-directory structure in Clean directory(/mnt/ssftp/staging) referencing Staging nested sub-directory structure. 
  <img src="./doc/ssftp-fileshare-sameuserdir.png" width="850" height="350" />  
  
* Below explains what each sSFTP directory is used for  
  <img src="./doc/ssftp-fileshare.png" width="700" height="600" />  

### How Things Work - Proposed Deployment Architecture

<img src="./doc/ssftp-azure-architecture.png" width="850" height="700" />  

* External Internet Sftp clients can download and upload files to and from sSFTP through Azure Firewall as Firewall supports Sftp protocol

* Clients from on-premise and peered-VNets can connect to sSFTP through a Private IP provided by Azure Container Instance deployed on VNet

* For ClamAV to receive signature updates and sSFTP to invoke Webhook, Route Table/UDR can be applied on sSFTP subnet to transparently route 0.0.0.0/0 to Firewall to reach any    Internet endpoints or other Azure services

* sSFTP communicates with ClamAV in an inter-process manner via TCP locahost:3310 within the same compute instance provided by Azure Container Instance

* Azure File Share ssftp-staging(/mnt/ssftp/staging) is mounted to both sSFTP and ClamAV containers so that clients can upload to same Staging directory that ClamAV reaches for scanning. 

* Downloading and processing clean files from sSFTP can be in the following ways:
    * Clean file share can be mounted to Pods in Azure Kubernetes Service and VMs
    * Clean file share can be mounted to App Service Linux
    * Azure Function deployed in App Service Environment can use any Sftp client library to connect to sSFTP through ACI Private IP
    * Daemons or jobs in VMs residing in  same or peered VNets can connect to sSFTP via ACI Private IP
    * SFTP Clients from the Internet can connect via Azure Firewall or Firewall of your choice to sSFTP via same ACI Private IP:Port and also using same "CleanDir SFTP Accounts"       to access directories in Clean file share

* Logging: currently supports logging to StdOut and files in Azure File. More log destinations coming soon...

### Configuring sSFTP  

Configurable is all done through a [single Yaml file](https://github.com/weixian-zhang/Azure-sSFTP/blob/main/deploy/ssftp.yaml).  
*ssftp.yaml must be located in mounted fileshare path as /mnt/ssftp/system/ssftp.yaml.
Update ssftp.yaml by uploading and overwriting Yaml file in ssftp-system fileshare, without restarting containers sSFTP monitors and load file changes from path: /mnt/ssftp/system/ssftp.yaml  

<img src="./doc/ssftp-config-update.png" width="500" height="300" />  

```yaml
sftpClientDownloaders:              #Downloaders are Sftp clients runs concurrently to download files from remote Sftp servers
  - name: "test.rebex.net-1"        #mandatory unique name
    host: "test.rebex.net"
    port: 22
    username: "demo"
    password: "password"
    privateKeyPath: "/mnt/ssftp/system/downloaders/rsa-putty-privatekey-authn.ppk"
    privatekeyPassphrase: "password-for-privatekey" #empty if no privatekey path is empty
    localStagingDirectory: "staging-rebex-1"  #files downloaded to "staging" directory /mnt/ssftp/staging/{localStagingDirectory}
    remoteDirectory: ""             #optional
    deleteRemoteFileAfterDownload: false
    overrideExistingFile: true
```  
* Supports multiple Sftp client Downloaders
* Downloaders save downloaded files to Staging directory /mnt/ssftp/staging
* localStagingDirectory - is a sub-directory in Staging /mnt/ssftp/staging/{localStagingDirectory}
* privateKeyPath - supports Putty or PEM RSA private key file to authenticate against remote Sft server that requires Public Key authn
* privatekeyPassphrase - the password that secures the Private Key
* remoteDirectory - Commonly, when Downloader logs-in to Sftp server, the server would have jailed this login account to a particular directory
  Unless you want to access a sub-directory under the remote jailed directory then specify the remote sub-directory name here
* deleteRemoteFileAfterDownload - sSFTP tries to delete remote file after download, throws error is permission is missing
* overrideExistingFile - true to override existing downloaded file with same file name


### Deploy sSFTP  
1. Prerequisites  
[Install Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)  

2. Create Network Profile for Azure Container Instance  

   2.1 Login to Azure  
        <code> az login </code>

   2.2 To deploy ACI into a VNet Subnet ACI needs a network profile, this network profile can then be reuse to deploy 1 or more future ACI Container Groups into the same Subnet.  
       The following command creates a temporary container instance in order to create a reusable network profile.  
        <code> az container create --resource-group <resource group> --name aci-temp-test-np --image alpine --vnet $vnetName --subnet $subnetName --restart-policy never </code>        <br />
       <br />
       Wait a moment for  "aci-temp-test-np" container to complete creation, then copy the <b>network profile id</b>  
   
     <img src="./doc/azcli-networkprofile.png" width="650" height="350" />
     <br />
     <br />
    2.3 Delete container "aci-temp-test-np" (we only need this container to get the network profile ID)
    <code> az container delete -g <resource group> -n aci-temp-test-np -y </code>  
   
3. Deploy sSFTP using Container Instance Yaml

    3.1 Save a copy of [sSFTP ACI Yaml file](https://raw.githubusercontent.com/weixian-zhang/Azure-sSFTP/main/deploy/deploy-aci-template.yaml) as "deploy-aci.yaml".  
        Replace all < values > with comment "input"  and save the file. Refer to the following ACI Yaml template.          
        
    3.2 Deploy yaml file by running the following command  
        <code> az container create -g <resource group> --file .\deploy-aci.yaml </code>

### Webhook  

sSFTP supports webhook when a virus is found, HTTP POST schema below:
```json
  {
    "username": "user1",
    "scanMessage": "Win.Test.EICAR_HDB-1 FOUND",
    "filePath": "/mnt/ssftp/quarantine/v.exe",
    "timeGenerated": "Tue Mar  9 05:50:06 2021"
  }
```

### Networking  
As ACI is deployed in a Subnet, you can choose to assign a User-Defined Route (UDR) to route all outbound traffic from sSFTP to an Azure Firewall or any NextGen Firewall.  
An example of Azure Firewall Application Rule with domains whitelisted for sSFTP to work.  
Also refer to [How it works](#behind-the-scenes-how-ssftp-works) for more details.  
<br />
<img src="./doc/azfw-app-rules.png" width="850" height="150" />  
<br />
<br />
