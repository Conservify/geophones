#!/bin/sh

/30 * * * * /app/resilience 8.8.8.8 | /usr/bin/logger -t resilience 2>&1
