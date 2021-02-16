## Azure Scanned SFTP  

* [What is sSFTP](#what-is-ssftp)
* [Deploy sSFTP](#deploy-ssftp)
* [How it works](#behind-the-scenes-how-ssftp-works)

### What is sSFTP
Azure sSFTP (Scanned SFTP) is a PaaS solution thats provides SFTP server with integrated [ClamAV](https://www.clamav.net/) virus scanning and Azure File as the file storage.  
sSFTP leverages Azure Container Instance to host 3 containers into a single Container Group namely
* [SFTP Server (by atmoz)](https://hub.docker.com/r/atmoz/sftp/) listening to port 22
* [ClamAV (by mkodockx) container](https://hub.docker.com/r/mkodockx/docker-clamav/) with selfupdate of virus signature and Clamd (daemon) listening to port 3310.
* [sSFTP (by Me) container](https://hub.docker.com/repository/docker/wxzd/ssftp) watches for uploaded files, sends file for scanning and sort files to appropriate mounted directories differentiating clean and virus-detected files.

This solution favours the deployment of Container Instance into VNet-Subnet as most Enterprise based solutions practice similar 
### Deploy sSFTP  
1. Prerequisites  
[Install Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)  

2. Create Network Profile for Azure Container Instance  

   2.1 Login to Azure  
        <code> az login </code>

   2.2 To deploy ACI into a VNet Subnet ACI needs a network profile, this network profile can then be reuse to deploy 1 or more future ACI Contaier Groups.  
       The following command creates a temporary container instance in order to create a reusable network profile.  
        <code> az container create --resource-group <resource group> --name aci-temp-test-np --image alpine --vnet $vnetName --subnet $subnetName --restart-policy never </code>  
       Wait a moment for container "aci-temp-test-np" to complete creation, and copy the <b>network profile id</b>


### Behind the Scenes How sSFTP Works




https://docs.microsoft.com/en-us/azure/container-instances/container-instances-region-availability
