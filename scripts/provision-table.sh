#!/bin/bash
set -e
# create the tasks table using Go helper
# requires Go 1.24+

go run ./api/provision
