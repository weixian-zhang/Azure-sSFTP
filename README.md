## Azure Scanned SFTP  

* [What is sSFTP](#what-is-ssftp)
* [Deploy sSFTP](#deploy-ssftp)
* [How it works](#behind-the-scenes-how-ssftp-works)

#### What is sSFTP
Azure sSFTP (Scanned SFTP) is a PaaS solution thats provides SFTP server with integrated [ClamAV](https://www.clamav.net/) virus scanning and Azure File as the file storage. sSFTP leverages Azure Container Instance to host 3 containers into a single Container Group namely [SFTP Server (by atmoz)](https://hub.docker.com/r/atmoz/sftp/) listening to port 22, [ClamAV (by mkodockx) container](https://hub.docker.com/r/mkodockx/docker-clamav/) with self update of virus signature and Clamd (daemon) listening to port 3310, and lastly sSFTP (by Weixian) daemon watches for uploaded files, sends file for scanning and sort files to appropriate mounted directories differentiating clean and virus-detected files.

#### Setup & Usage  



#### Behind the Scenes How sSFTP Works




https://docs.microsoft.com/en-us/azure/container-instances/container-instances-region-availability
