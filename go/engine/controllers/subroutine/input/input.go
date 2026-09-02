// Package input is the live actions/config console controller. Execute parses a
// single command line and runs it against the running server: inspect/clear
// session caches, run read-only DB queries, grant/revoke user rights, request a
// database dump, or run a (whitelisted) local firewall command. It is used both
// by ReadCommand (stdin, for CLI debugging) and by the admin-only console page
// (HTTP), so all privilege checks live at the caller.
package input

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"tls-rest/go/engine/controllers/auth"
	"tls-rest/go/engine/controllers/db/cache"
	"tls-rest/go/engine/controllers/db/pgdb"
)

// Execute runs one console command and returns its textual output. Errors are
// returned so the caller can surface them; partial output may accompany an error.
func Execute(line string) (string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}
	parts := strings.Fields(line)
	cmd, args := parts[0], parts[1:]

	switch cmd {
	case "help", "?":
		return helpText(), nil
	case "cache":
		return cacheCmd(args)
	case "db":
		return dbCmd(args, line)
	case "rights":
		return rightsCmd(args)
	case "fw", "firewall":
		return firewallCmd(args)
	default:
		return "", fmt.Errorf("unknown command %q — try `help`", cmd)
	}
}

func helpText() string {
	return strings.Join([]string{
		"commands:",
		"  cache list                       list session cache keys + count",
		"  cache clear                      drop all sessions",
		"  cache drop <key>                 drop one session",
		"  cache bump-rights                invalidate cached rights (force re-resolve)",
		"  db query <SELECT ...>            run a read-only query",
		"  db dump [table]                  pg_dump the database (or one table)",
		"  rights grant <userID> <module> <modes>   grant a user module modes (bitmask)",
		"  rights revoke <userID> <module>          revoke a user's per-user rights",
		"  fw <status|allow|deny> [args]    local firewall (whitelisted)",
	}, "\n")
}

// --- cache ---------------------------------------------------------------

func cacheCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: cache <list|clear|drop <key>|bump-rights>")
	}
	c := cache.SessionCacheInstance
	switch args[0] {
	case "list":
		keys := c.Keys()
		return fmt.Sprintf("%d session(s):\n%s", len(keys), strings.Join(keys, "\n")), nil
	case "clear":
		n := c.Len()
		c.Clear()
		return fmt.Sprintf("cleared %d session(s)", n), nil
	case "drop":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: cache drop <key>")
		}
		c.Delete(args[1])
		return "dropped " + args[1], nil
	case "bump-rights":
		auth.BumpRightsEpoch()
		return "rights epoch bumped — sessions will re-resolve on next request", nil
	default:
		return "", fmt.Errorf("unknown cache subcommand %q", args[0])
	}
}

// --- db ------------------------------------------------------------------

func dbCmd(args []string, full string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: db <query <SELECT ...>|dump [table]>")
	}
	switch args[0] {
	case "query":
		sql := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(full), "db query"))
		if sql == "" {
			return "", fmt.Errorf("usage: db query <SELECT ...>")
		}
		// Read-only guard: only allow SELECT/EXPLAIN/WITH.
		low := strings.ToLower(strings.TrimLeft(sql, "( \t"))
		if !(strings.HasPrefix(low, "select") || strings.HasPrefix(low, "explain") || strings.HasPrefix(low, "with")) {
			return "", fmt.Errorf("only read-only queries (SELECT/EXPLAIN/WITH) are allowed here")
		}
		db, err := pgdb.GetInstance()
		if err != nil {
			return "", err
		}
		rows, err := db.RQuery(sql)
		if err != nil {
			return "", err
		}
		out, _ := json.MarshalIndent(rows, "", "  ")
		return fmt.Sprintf("%d row(s):\n%s", len(rows), string(out)), nil
	case "dump":
		return dbDump(args[1:])
	default:
		return "", fmt.Errorf("unknown db subcommand %q", args[0])
	}
}

// dbDump shells out to pg_dump using the DATABASE_URL/PG* environment. Returns
// the dump path. Guard access at the caller (admin only).
func dbDump(args []string) (string, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "", fmt.Errorf("set DATABASE_URL for pg_dump")
	}
	outPath := fmt.Sprintf("/tmp/dump-%d.sql", os.Getpid())
	cmdArgs := []string{dsn, "-f", outPath}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, "-t", args[0])
	}
	c := exec.Command("pg_dump", cmdArgs...)
	if b, err := c.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pg_dump failed: %s: %w", strings.TrimSpace(string(b)), err)
	}
	return "dump written to " + outPath, nil
}

// --- rights --------------------------------------------------------------

func rightsCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: rights <grant <userID> <module> <modes>|revoke <userID> <module>>")
	}
	db, err := pgdb.GetInstance()
	if err != nil {
		return "", err
	}
	switch args[0] {
	case "grant":
		if len(args) < 4 {
			return "", fmt.Errorf("usage: rights grant <userID> <module> <modes>")
		}
		uid, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid userID")
		}
		modes, err := strconv.Atoi(args[3])
		if err != nil {
			return "", fmt.Errorf("invalid modes (integer bitmask)")
		}
		if _, err := db.Exec(
			`INSERT INTO user_rights (user_id, module, modes) VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, module) DO UPDATE SET modes = EXCLUDED.modes`,
			uid, args[2], modes); err != nil {
			return "", err
		}
		auth.BumpRightsEpoch()
		return fmt.Sprintf("granted user %d modes %d on %q", uid, modes, args[2]), nil
	case "revoke":
		if len(args) < 3 {
			return "", fmt.Errorf("usage: rights revoke <userID> <module>")
		}
		uid, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid userID")
		}
		if _, err := db.Exec(`DELETE FROM user_rights WHERE user_id = $1 AND module = $2`, uid, args[2]); err != nil {
			return "", err
		}
		auth.BumpRightsEpoch()
		return fmt.Sprintf("revoked user %d rights on %q", uid, args[2]), nil
	default:
		return "", fmt.Errorf("unknown rights subcommand %q", args[0])
	}
}

// --- firewall ------------------------------------------------------------

// firewallCmd runs a whitelisted local firewall action. This is intentionally
// conservative — extend allowed[] for your platform (ufw/pfctl/iptables) and
// keep access admin-only. It shells out; never pass raw user strings unguarded.
func firewallCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: fw <status|allow|deny> [args]")
	}
	// Map a small set of safe verbs to concrete commands (ufw shown as example).
	switch args[0] {
	case "status":
		return runFirewall("ufw", "status")
	case "allow":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: fw allow <port|service>")
		}
		return runFirewall("ufw", "allow", args[1])
	case "deny":
		if len(args) < 2 {
			return "", fmt.Errorf("usage: fw deny <port|service>")
		}
		return runFirewall("ufw", "deny", args[1])
	default:
		return "", fmt.Errorf("firewall verb %q not permitted", args[0])
	}
}

func runFirewall(name string, a ...string) (string, error) {
	out, err := exec.Command(name, a...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s failed: %w", name, err)
	}
	return string(out), nil
}

// ReadCommand runs an interactive console on stdin (CLI debugging). Each line is
// passed to Execute and its output printed.
func ReadCommand() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		out, cerr := Execute(line)
		if cerr != nil {
			fmt.Println("error:", cerr)
		}
		if out != "" {
			fmt.Println(out)
		}
	}
}
