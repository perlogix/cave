#!/bin/sh

echo $RANDOM | md5sum | awk '{print $1}' > /etc/machine-id
./bunker "$@"