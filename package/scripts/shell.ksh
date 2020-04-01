if [ -z "$DOCKER_HOST" ]; then
   echo "ERROR: no DOCKER_HOST defined"
   exit 1
fi

echo "*****************************************"
echo "running on $DOCKER_HOST"
echo "*****************************************"

# set the definitions
INSTANCE=virgo4-pool-worldcat-ws
NAMESPACE=uvadave

# environment attributes
DOCKER_ENV=""
docker run -it -p 8180:8080 $DOCKER_ENV $NAMESPACE/$INSTANCE /bin/bash -l

# return status
exit $?
