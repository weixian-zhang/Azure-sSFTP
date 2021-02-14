docker network create vnet

docker run --name clamav -d -p 3310:3310 --net vnet mkodockx/docker-clamav 

docker run --net vnet --name ssftp --env stagingPath=/home/ssftp/staging --env cleanPath=/home/ssftp/clean --env quarantinePath=/home/ssftp/quarantine --env errorPath=/home/ssftp/error --env logPath=/home/ssftp/log wxzd/ssftp:1.0 