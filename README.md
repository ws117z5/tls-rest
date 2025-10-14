Tests

go test ./...

Coverage

t="/tmp/go-cover.$$.tmp"
go test -coverprofile=$t $@ && go tool cover -html=$t && unlink $t

DB init 

init/init_all.zsh

Backup

backup.zsh