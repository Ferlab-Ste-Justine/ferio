#!/bin/bash
echo "Waiting to be terminated..."
while sleep 1
do
    trap "echo 'terminating...' && sleep 5 && exit 0" TERM
done