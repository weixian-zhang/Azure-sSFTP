az login

$containerInstanceName = "aci-ssftp-gut-intranetzone"
$rg = "rgGCCSHOL"
$vnetName = "vnetIntranetZone"
$subnetName = "GUT-SSFTP"

# copy network profile id
az container create --resource-group $rg `
--name temp-create-netprofile --image alpine:3.5 --restart-policy never `
--command-line "wget $CONTAINER_GROUP_IP" `
--vnet $vnetName --subnet $subnetName

az container delete -g $rg -n temp-create-netprofile -y