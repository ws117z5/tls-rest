#!/bin/zsh

# Run pgdb service
echo "Restarting pgdb service:"
brew services restart postgresql

# Run reddis
echo "Restarting redis:"
brew services restart redis

# run db structure
go run init.go