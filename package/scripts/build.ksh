#if [ -z "$DOCKER_HOST" ]; then
#   echo "ERROR: no DOCKER_HOST defined"
#   exit 1
#fi

echo "*****************************************"
echo "building on $DOCKER_HOST"
echo "*****************************************"

if [ -z "$DOCKER_HOST" ]; then
   DOCKER_TOOL=docker
else
   DOCKER_TOOL=docker-legacy
fi

# set the definitions
INSTANCE=virgo4-pool-worldcat-ws
NAMESPACE=uvadave

# build the image
$DOCKER_TOOL build -f package/Dockerfile --build-arg BUILD_TAG=12345 -t $NAMESPACE/$INSTANCE .

# return status
exit $?
