# tls-rest

A Go + React module engine: declare a module's fields once, in Go, and get a CRUD REST API, an admin UI, row- and field-level rights, list filters, and search for free. `go/regisrty.go` registers everything - engine modules (users, images, comments, likes, access control, actions, logging, statistics) and app modules (posts, the papers game with peer-to-peer WebRTC video) alike.

## Manuals

Full write-ups, in the order you'd actually want them:

| Manual | What it covers |
|---|---|
| [Project Init](https://koroteev.dev/posts/388) | Clone to running server: `.env`, `go.config.json`, database setup, Docker vs. local. |
| [Fieldset Field Properties](https://koroteev.dev/posts/2) | Every `Field` type, mode, and builder option (`go/engine/controllers/field/field.go`). |
| [Module Declaration Example](https://koroteev.dev/posts/390) | One fully-worked module exercising nearly every engine option, with diagrams. |
| [User / User Group / Rights System](https://koroteev.dev/posts/389) | How groups, `user_group_rights`/`user_rights`, and row/field visibility resolve. |
| [P2P Load Balancing](https://koroteev.dev/posts/1) | The papers game's peer-to-peer WebRTC mesh. |

The same source lives in this repo under `markdown/*.txt`, kept in sync with the posts above.

## Development

**Tests**
```sh
go test ./...
```

**Coverage**
```sh
t="/tmp/go-cover.$$.tmp"
go test -coverprofile=$t $@ && go tool cover -html=$t && unlink $t
```

**Database backup**
```sh
init/backup.zsh
```

Setting up a database from scratch, or Docker vs. local run instructions? See the [Project Init](https://koroteev.dev/posts/388) manual - `init/init_all.zsh` is stale (macOS/Homebrew-specific, calls into a since-superseded `init.go`) and no longer the recommended path.
