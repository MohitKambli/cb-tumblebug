#!/bin/bash
source ../setup.env


#num=0
#for NAME in "${CONNECT_NAMES[@]}"
#for NAME in "${CONNECT_NAMES[0]}"
#do
#        curl -H "${AUTH}" -sX GET http://$RESTSERVER:1024/vmimage?connection_name=${NAME} |json_pp &
#        num=`expr $num + 1`
#done

curl -H "${AUTH}" -sX GET http://$TUMBLEBUG_IP:1323/tumblebug/ns/${NS_ID}/resources/image | json_pp &