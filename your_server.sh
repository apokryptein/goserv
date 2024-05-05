#!/bin/sh
# DON'T EDIT THIS!
set -e
tmpFile=$(mktemp)
go build -o "$tmpFile" app/*.go
exec "$tmpFile" "$@"
