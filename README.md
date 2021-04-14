## Azure Scanned SFTP  

* [What is sSFTP](#what-is-ssftp)
* [Features](#features)
* [How Things Work - Directories & Conventions](#how-things-work---directories--conventions)
* [How Things Work - Architecture](#how-things-work---architecture)
* [Configuring sSFTP](#configuring-ssftp)
* [Deploy sSFTP](#deploy-ssftp)
* [Webhook](#webhook)
* [Networking](#networking) 

### What is sSFTP
Azure sSFTP (Scanned SFTP) is a container-based solution leverages Azure Container Instance to provide SFTP server with integrated [ClamAV](https://www.clamav.net/) virus scanning and Azure File as the file storage.  
sSFTP consists of 2 containers into a single Container Group namely
* [ClamAV (by mkodockx) container](https://hub.docker.com/r/mkodockx/docker-clamav/) with selfupdating of virus signature and Clamd (daemon) listening to port 3310 for virus scan commands.
* [sSFTP container](https://hub.docker.com/repository/docker/wxzd/ssftp) runs a SFTP server, watches for uploaded files, scans and sort files into appropriate mounted directories to isolate clean and virus-detected files.  

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
  * FileWatcher module picks up files from Staging and send them for ClamAV scanning and sorting
  * Virus-free files determined by ClamAV are move to <b>Clean directory(/mnt/ssftp/clean)</b>
  * Files containing virus are move to <b>Quarantine directory(/mnt/ssftp/quarantine)</b>  
Above process is performed on each uploaded file.  
  
* The Downloader module are Sftp clients that downloads from remote Sftp server. You can configure multiple Downloaders through [ssftp.yaml](https://github.com/weixian-zhang/Azure-sSFTP/blob/main/deploy/ssftp.yaml) to support concurrent downloads from remote Sftp servers.  
  <b>*Downloaded files are save to Staging directory(/mnt/ssftp/staging) for FileWatcher to scan and sort.</b>
  
* Similarly to Downloaders, Uploaders are Sftp clients that uploads files to remote Sftp servers and supports multiple Uploaders running concurrently.   
  <b>*Uploaders only upload files from Clean directory(/mnt/ssftp/clean), nested directories in Clean directory are supported</b>

* FileWatcher simply watches all files in nested directories in Staging directory(/mnt/ssftp/staging), picks up files sending them to scan and sort. As FileWatcher sort files to Clean directory, it will create the same nested directory structure in Clean directory(/mnt/ssftp/staging) following Staging directory structure. 
  <img src="./doc/ssftp-fileshare-sameuserdir.png" width="850" height="350" />  
  
* Below explains what each sSFTP directory is used for  
  <img src="./doc/ssftp-fileshare.png" width="700" height="500" />  

### How Things Work - Architecture

<img src="./doc/ssftp-azure-architecture.png" width="850" height="700" />  

#### Networking Facts

* External Sftp clients can download and upload files to and from sSFTP through Azure Firewall as Firewall supports Sftp protocol.

* For ClamAV to receive signature updates and sSFTP to invoke Webhook, Route Table/UDR can be applied to sSFTP subnet and route 0.0.0.0/0 to Firewall to reach any Internet or   Azure services

* sSFTP communicates with ClamAV via TCP locahost:3310 within the same compute instance provided by Azure Container Instance

* Clients from peered-VNets can connect to sSFTP through a Private IP provided by Azure Container Instance. E.g: ACI-PrivateIP:2002

SFTP clients upload files into their designated directory "/mnt/ssftp/<b>staging</b>/{designated directory}" as configured in [ssftp.yaml](#configuring-ssftp), 
  configured directory will be auto created when user/client logins.   
  sSFTP picks up the uploaded file and sends a command to ClamD (ClamAV scan daemon) running in ClamAV container in the same Azure Container Instance Container Group.  
  If the scan result is good, sSFTP moves file to the Clean directory /mnt/ssftp/<b>clean</b>/{same name as Staging designated directory}.  
  If ClamaV detects virus, sSFTP then moves file into Quarantine directory /mnt/ssftp/<b>quarantine</b>/{same name as Staging designated directory}  

* Azure File Share ssftp-staging is mounted to both sSFTP and ClamAV containers so that clients can upload to same share that ClamAV reaches for scanning.  

* Reading and processing clean files from sSFTP can be in the following ways:
    * Clean file share can be mounted to Pods in Azure Kubernetes Service and VMs
    * Other apps hosted in App Service, Function or VMs in the same or peered VNets can connect to sSFTP via ACI Private IP:Port, using "CleanDir SFTP Accounts" to access
      directories in Clean file share
    * SFTP Clients from the Internet can connect via Azure Firewall or Firewall of your choice to sSFTP via same ACI Private IP:Port and also using same "CleanDir SFTP Accounts"       to access directories in Clean file share

* ClamAV updates its database through the Internet where traffic can be forward-proxied to Firewall using [Azure User Defined Route](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-udr-overview#user-defined)

* sSFTP supports webhook when any virus is detected, webhook HTTP POST call can also be forward-proxied to Firewall using UDR

* Logging: currently supports logging to StdOut and files in Azure File. More log destinations coming soon...
      
### Configuring sSFTP  

Configurable is all done through a [single Yaml file](https://github.com/weixian-zhang/Azure-sSFTP/blob/main/deploy/ssftp.yaml) and file must be located in mounted fileshare path: /mnt/ssftp/system/ssftp.yaml.
Update ssftp.yaml by uploading and overwriting Yaml file in ssftp-system fileshare, without restarting containers sSFTP monitors and load file changes from path: /mnt/ssftp/system/ssftp.yaml  

<img src="./doc/ssftp-config-update.png" width="500" height="300" />  

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
        
     ```yaml
      apiVersion: 2019-12-01
      location: southeastasia                         #input: Azure resource location
      name: aci-ssftp                                 #input: ACI name or just leave this default
      properties:
        containers:

        - name: clamav
          properties:
            image: mkodockx/docker-clamav:alpine
            environmentVariables:
            - name: CLAMD_CONF_FILE
              value: /mnt/ssftp/system/clamd.conf     #Note: path by convention, do not change path
            resources:
              requests:
                cpu: 2
                memoryInGb: 8
            ports:
            - port: 3310
            volumeMounts:                             #Note: path by convention, do not change path
             - mountPath: /mnt/ssftp/system
               name: fs-system
               readOnly: false
             - mountPath: /mnt/ssftp/staging
               name: fs-staging
               readOnly: false

        - name: ssftp
          properties:
            image: wxzd/ssftp:1.0
            resources:
              requests:
                cpu: 2
                memoryInGb: 8
            ports:
              - port: 2002                             #input: port must match ipAddress.ports.port below & "sftpPort" in ssftp.yaml
            volumeMounts:                              #Note: all mountPaths by convention, do not change path
            - mountPath: /mnt/ssftp/staging
              name: fs-staging
              readOnly: false
            - mountPath: /mnt/ssftp/clean
              name: fs-clean
              readOnly: false
            - mountPath: /mnt/ssftp/quarantine
              name: fs-quarantine
              readOnly: false
            - mountPath: /mnt/ssftp/error
              name: fs-error
              readOnly: false
            - mountPath: /mnt/ssftp/log                 #input - optional, if exist to match LogSink.Path in ssftp.yaml
              name: fs-log
              readOnly: false
            - mountPath: /mnt/ssftp/system
              name: fs-system
              readOnly: false
        volumes:
        - name: fs-staging
          azureFile:
            sharename: ssftp-staging                    #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input
        - name: fs-clean
          azureFile:
            sharename: ssftp-clean                      #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input
        - name: fs-quarantine
          azureFile:
            sharename: ssftp-quarantine                 #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input
        - name: fs-error
          azureFile:
            sharename: ssftp-error                      #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input
        - name: fs-log
          azureFile:
            sharename: ssftp-log                        #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input
        - name: fs-system
          azureFile:
            sharename: ssftp-system                     #Note: file share name can be different
            storageAccountName: <storage accountn name> #input
            storageAccountKey: <storage account key>    #input

        ipAddress:
          type: Private
          ports:
          - protocol: tcp
            port: 2002                                  #input: port must match container port above & "sftpPort" in ssftp.yaml
        networkProfile:
          id: <network profile resource ID>             #input
        restartPolicy: Always
        osType: Linux
      tags: null
      type: Microsoft.ContainerInstance/containerGroups
     ```
        
        
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
