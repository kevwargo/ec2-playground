#!/bin/bash

if [ "$(source /etc/os-release && echo $NAME)" == "Red Hat Enterprise Linux" ]; then
    command -v python3 || yum install -y python3
fi
