az login

rem copy network profile id
az container create --resource-group rgGCCSHOL ^
--name temp-create-netprofile --image alpine:3.5 --restart-policy never ^
--command-line "wget $CONTAINER_GROUP_IP" ^
--vnet vnetInternetZone --subnet Subnet-sSFTP

az container delete -g rgGCCSHOL -n temp-create-netprofile -y

az container create --resource-group rgGCCSHOL ^
--file deploy-aci.yaml

rem view deployment events
 az container show -g rgGCCSHOL -n aci-containergroup-ssftp