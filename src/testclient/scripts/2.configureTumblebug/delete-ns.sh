#!/bin/bash

#function delete_ns() {


    TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Namespace: Delete"
    echo "####################################################################"

    INDEX=${1}

    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID | jq ''
    echo ""
#}

#delete_ns