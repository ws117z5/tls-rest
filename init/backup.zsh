#!/bin/zsh

#dormat date
date_string=$(date +'%Y-%m-%d %H:%M:%S')

echo $date_string.sql

pg_dump -U ws117z5 -d tls-rest -f "backup/$date_string.sql"
