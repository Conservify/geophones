#!/bin/sh

/app/morningstar --csv /app/data/morningstar-glacier.csv | /usr/bin/logger -t morningstar 2>&1
