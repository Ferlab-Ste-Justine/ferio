#!/bin/bash
echo "Waiting to be terminated..."
trap "sleep 5 && echo 'terminating' && exit 0" TERM

while true
do
    sleep 5
done