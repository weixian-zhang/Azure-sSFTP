## Azure Scanned SFTP  

* [What is sSFTP](#what-is-ssftp)
* [Features](#features)
* [How things Work](#how-things-work)
* [AzFile, Folder Structure & Conventions](#azure-file-share-structure-directory-structure-&-conventions)
* [Configuring sSFTP](#configuring-ssftp)
* [Deploy sSFTP](#deploy-ssftp)
* [Networking](#networking) 

### What is sSFTP
Azure sSFTP (Scanned SFTP) is a container-based solution leverages Azure Container Instance to provide SFTP server with integrated [ClamAV](https://www.clamav.net/) virus scanning and Azure File as the file storage.  
sSFTP consists of 2 containers into a single Container Group namely
* [ClamAV (by mkodockx) container](https://hub.docker.com/r/mkodockx/docker-clamav/) with selfupdating of virus signature and Clamd (daemon) listening to port 3310 for virus scan commands.
* [sSFTP container](https://hub.docker.com/repository/docker/wxzd/ssftp) runs a SFTP server, watches for uploaded files, scans and sort files into appropriate mounted directories to isolate clean and virus-detected files.  

### Features  

* Container-based solution that runs on Azure Container Instance (PaaS), no infrastructure maintainence needed
* sSFTP's runs securely in Virtual Network while Internet traffic to SFTP server is proxied through Azure Firewall or Firewall of your choice
* SFTP server integrated, no separate product license needed
* ClamAV virus scan integrated
* Supports certificate and password authentication
* Azure File as the file storage for SFTP server
* Supports Webhook invocation when virus is detected per file
* Each SFTP user/service login account is rooted to its configured directory only
* Supports multi-user/service accounts per each root directory
* Add or remove user/service accounts without restarting SFTP server
* Easy configuration using a single Yaml file
* Yaml config changes is recognized on-the-fly with no restart needed
* For whatever reason if sSFTP's Container Instance is restarted or removed, files are still retained in Azure File


### How Things Work

SFTP clients upload files into their designated directory "/mnt/ssftp/<b>staging</b>/{designated directory}" as configured in [ssftp.yaml](#configuring-ssftp),  
sSFTP picks up the uploaded file and sends a command to ClamD (ClamAV scan daemon) running in ClamAV container in the same Azure Container Instance Container Group.  
If the scan result is good, sSFTP moves file to the Clean directory /mnt/ssftp/<b>clean</b>/{same name as Staging designated directory}.  
If ClamaV detects virus, sSFTP then moves file into Quarantine directory /mnt/ssftp/<b>quarantine</b>/{same name as Staging designated directory}  

Azure File Share ssftp-staging is mounted to both sSFTP and ClamAV containers so that clients can upload to same share that ClamAV can reach for scanning.

<img src="./doc/ssftp-azure-architecture.png" width="850" height="750" />

### Azure File Share Structure, Directory Structure & Conventions  

The following file shares are required by convention (file share name can be changed),  
except for "ssftp-log" where sSFTP writes log files to which is optional.  


<img src="./doc/ssftp-fileshare.png" width="650" height="450" />  
<br />
An example depicting folder structure in Staging and Clean file share are identical  
<img src="./doc/ssftp-fileshare-sameuserdir.png" width="850" height="300" />
      
### Configuring sSFTP  


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
   
     <img src="./doc/azcli-networkprofile.png" width="750" height="450" />
     <br />
     <br />
    2.3 Delete container "aci-temp-test-np"
    <code> az container delete -g <resource group> -n aci-temp-test-np -y </code>  
   
3. Deploy sSFTP using Container Instance Yaml

    3.1 Save a copy of [sSFTP ACI Yaml file](https://raw.githubusercontent.com/weixian-zhang/Azure-sSFTP/main/deploy/deploy-aci-template.yaml) as "deploy-aci.yaml".  
        Replace all < values > in this file and save the file. Refer to the following screenshots.  
        
      <img src="./doc/aci-template-1.png" width="650" height="450" />  
      <br />
      <img src="./doc/aci-template-2.png" width="550" height="400" />
      <br />
      <br />
      Webhook Url is optional.  
      Once sSFTP detects virus in a file, this Wwebhook Url is invoked which allows sSFTP to integrate with Azure Logic App, Function and custom apps for many other possibilities.
      <br />
      <br />
      <img src="./doc/aci-template-3.png" width="550" height="400" />  
      <br />
      <img src="./doc/aci-template-4.png" width="550" height="400" />  
      <br />
        
        
    3.2 Deploy yaml file by running the following command  
        <code> az container create -g <resource group> --file .\deploy-aci.yaml </code>

### Networking  
As ACI is deployed in a Subnet, you can choose to assign a User-Defined Route (UDR) to route all outbound traffic from sSFTP to an Azure Firewall or any NextGen Firewall.  
An example of Azure Firewall Application Rule with domains whitelisted for sSFTP to work.  
Also refer to [How it works](#behind-the-scenes-how-ssftp-works) for more details.  
<br />
<img src="./doc/azfw-app-rules.png" width="850" height="150" />  
<br />
<br />
