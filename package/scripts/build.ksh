if [ -z "$DOCKER_HOST" ]; then
   echo "ERROR: no DOCKER_HOST defined"
   exit 1
fi

echo "*****************************************"
echo "building on $DOCKER_HOST"
echo "*****************************************"

# set the definitions
INSTANCE=virgo4-pool-worldcat-ws
NAMESPACE=uvadave

# build the image
docker build -f package/Dockerfile --build-arg BUILD_TAG=12345 -t $NAMESPACE/$INSTANCE .

# return status
exit $?
