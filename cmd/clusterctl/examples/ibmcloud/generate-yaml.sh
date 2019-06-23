#!/bin/bash
set -e

# Function that prints out the help message, describing the script
print_help()
{
  echo "$SCRIPT - generates a provider-configs.yaml file"
  echo ""
  echo "Usage:"
  echo "$SCRIPT [options] <path/to/clouds.yaml> <provider os: [ubuntu]> [output folder]"
  echo "options:"
  echo "-h, --help                    show brief help"
  echo "-f, --force-overwrite         if file to be generated already exists, force script to overwrite it"
  echo ""
}

# Supported Operating Systems
declare -a arr=("ubuntu")
SCRIPT=$(basename $0)
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            print_help
            exit 0
            ;;
          -f|--force-overwrite)
            OVERWRITE=1
            shift
            ;;
          *)
            break
            ;;
        esac
done

# Check if clouds.yaml file provided
if [[ -n "$1" ]] && [[ $1 != -* ]] && [[ $1 != --* ]];then
  CLOUDS_PATH="$1"
else
  echo "Error: No clouds.yaml provided"
  echo "You must provide a valid clouds.yaml"
  echo ""
  print_help
  exit 1
fi

# Check that OS is provided
if [[ -n "$2" ]] && [[ $2 != -* ]] && [[ $2 != --* ]]; then
  USER_OS=$(echo $2 | tr '[:upper:]' '[:lower:]')
else
  echo "Error: No provider OS specified"
  echo "You mush choose between the following operating systems: ubuntu"
  echo ""
  print_help
  exit 1
fi

OUTPUT=out
if [[ -n "$3" ]] && [[ $3 != -* ]] && [[ $3 != --* ]]; then
  OUTPUT=$(echo $3 | tr '[:upper:]' '[:lower:]')
else
  echo "no output folder provided, use name 'out' by default"
fi

# Check that OS is supported
for i in "${arr[@]}"
do
  if test "$USER_OS" = "$i"; then
    PROVIDER_OS=$i
    break
  fi
done

if test -z "$PROVIDER_OS"; then
  echo "provider-os error: $USER_OS is not one of the supported operating systems!"
  print_help
  exit 1
fi

if [ -e $OUTPUT/provider-components.yaml ] && [ "$OVERWRITE" != "1" ]; then
  echo "Can't overwrite provider-components.yaml without user permission. Either run the script again"
  echo "with -f or --force-overwrite, or delete the file in the out/ directory."
  echo ""
  print_help
  exit 1
fi

kubectlversion=`kubectl version | grep 'Minor:"[0-9]*' -o| grep -o [0-9]*`
for item in $kubectlversion; 
do 
   if [ $item -lt 14 ] ; 
   then 
       echo "kubectl client and server version must equal or bigger than 1.14";
       exit 1 
   fi; 
done

# Define global variables
PWD=$(cd `dirname $0`; pwd)
CONFIG_DIR=$PWD/provider-component/clouds-secrets/configs
OVERWRITE=${OVERWRITE:-0}
CLOUDS_PATH=${CLOUDS_PATH:-""}
USERDATA=$PWD/provider-component/user-data
MASTER_USER_DATA=$USERDATA/$PROVIDER_OS/templates/master-user-data.sh
WORKER_USER_DATA=$USERDATA/$PROVIDER_OS/templates/worker-user-data.sh

# Set default
CLOUD_SSH_PRIVATE_FILE=${HOME}/.ssh/id_ibmcloud
# Read SSH private file from env
if [ "x$IBMCLOUD_HOST_SSH_PRIVATE_FILE" != "x" ]; then
  CLOUD_SSH_PRIVATE_FILE=$IBMCLOUD_HOST_SSH_PRIVATE_FILE
fi

# Create ssh key to access IBM Cloud machines on demand
if [ ! -f ${CLOUD_SSH_PRIVATE_FILE} ]; then
  echo "Generating SSH key files for IBM cloud machine access."
  # This is needed because GetKubeConfig assumes the key in the home .ssh dir.
  ssh-keygen -t rsa -f ${CLOUD_SSH_PRIVATE_FILE}  -N ""
fi

# Prepare dependecies for kustomize
mkdir -p $CONFIG_DIR
cat $PWD/$CLOUDS_PATH > $CONFIG_DIR/clouds.yaml
cat "$MASTER_USER_DATA" > $USERDATA/$PROVIDER_OS/master-user-data.sh
cat "$WORKER_USER_DATA" > $USERDATA/$PROVIDER_OS/worker-user-data.sh

# Set up the output dir if it does not yet exist
mkdir -p $PWD/$OUTPUT
cp -n $PWD/cluster.yaml.template $PWD/$OUTPUT/cluster.yaml || true
cp -n $PWD/machines.yaml.template $PWD/$OUTPUT/machines.yaml || true

# Build provider-components.yaml with kustomize
kubectl kustomize $PWD/../../../../config > $PWD/$OUTPUT/provider-components.yaml

echo "---" >> $PWD/$OUTPUT/provider-components.yaml
kubectl kustomize $PWD/provider-component/clouds-secrets >> $PWD/$OUTPUT/provider-components.yaml


#latest kustomize do not allow include files outside the build folder, copy temply
cp -r $CONFIG_DIR/../../../../../../../vendor/sigs.k8s.io/cluster-api/config $PWD/provider-component/cluster-api
echo "---" >> $PWD/$OUTPUT/provider-components.yaml
kubectl kustomize $PWD/provider-component/cluster-api >> $PWD/$OUTPUT/provider-components.yaml
rm -fr $PWD/provider-component/cluster-api/config

echo "---" >> $PWD/$OUTPUT/provider-components.yaml
kubectl kustomize $USERDATA/$PROVIDER_OS >> $PWD/$OUTPUT/provider-components.yaml

