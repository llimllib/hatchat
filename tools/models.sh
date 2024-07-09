#!/usr/bin/env bash

# remove the xo database if it exists, and create it from the schema
rm -f xo.db
sqlite3 'xo.db' < schema.sql

# generate models from the schema
xo -v schema sqlite://xo.db -o server/xomodels

# generate custom queries
xo query sqlite://xo.db -M -B -2 -1 -o server/xomodels -T DefaultRoom << ENDSQL
SELECT ID AS id
FROM rooms r
WHERE r.is_default
ENDSQL

# remove the xo database
rm xo.db
