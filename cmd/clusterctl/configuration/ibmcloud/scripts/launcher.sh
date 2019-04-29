#!/bin/bash

set -x

USER_DATA_PATH=ibm_cloud_data
USER_DATA_FILE=${USER_DATA_PATH}/openstack/latest/user_data
mkdir ibm_cloud_data
if [ $? -ne 0 ]; then
    echo "Failed creating directory ${USER_DATA_PATH} under `pwd`."
    exit -1
fi

mount /dev/xvdh1 ibm_cloud_data/
if [ $? -ne 0 ]; then
    echo "Failed mounting /dev/xvdh1 to `pwd`/${USER_DATA_PATH}."
    exit -1
fi

if [ ! -f "${USER_DATA_FILE}" ];then
    echo "Failed mounting /dev/xvdh1 to `pwd`/${USER_DATA_PATH}."
    exit -1
fi

#base64 -d ibm_cloud_data/openstack/latest/user_data > ~/deployk8s.sh
cp ibm_cloud_data/openstack/latest/user_data /deployk8s.sh

# IBM cloud runs executable files automatically based on test
# To ensure the file is executed, explicitly run it

bash -x /deployk8s.sh
