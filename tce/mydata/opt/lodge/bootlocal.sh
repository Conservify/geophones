#!/bin/sh

/app/tunneller-wrapper --syslog tunneller-ssh --remote-port 7003 --log /var/log/tunneller.log --key /home/tc/.ssh/id_rsa --server 34.201.197.136 &