package constants

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Environment / secret configuration loader.
//
// Secrets that used to live in go/constants/private.go are now read from a
// .env file (or real OS environment variables). Resolution order for any key:
//   1. a real OS environment variable, if set and non-empty
//   2. a matching entry in the first .env file found (see envCandidates)
//   3. the fallback value passed to the getter
//
// The parser is intentionally dependency-free (standard library only) so it
// adds nothing to go.mod.

var (
	envOnce   sync.Once
	envValues = map[string]string{}
)

// envCandidates returns the .env locations to try, in priority order. The
// canonical location is {projectRoot}/.private/.env — ".private/" is already
// git-ignored, so it never risks being committed. Because the server can be
// launched from a directory other than the project root, the root is located
// by walking up to the nearest go.mod rather than trusting the process's
// current working directory.
func envCandidates() []string {
	var paths []string

	// 1. Explicit override, for deployments that place the file elsewhere.
	if p := os.Getenv("DOTENV_PATH"); p != "" {
		paths = append(paths, p)
	}

	// 2. Project-root-anchored locations (robust regardless of the CWD).
	if root := projectRoot(); root != "" {
		paths = append(paths,
			filepath.Join(root, ".private", ".env"),
			filepath.Join(root, ".env"),
		)
	}

	// 3. Last-resort CWD-relative fallbacks (works when launched from the root,
	//    matching how the app already resolves ./.private/ and ./css/).
	paths = append(paths, filepath.Join(".private", ".env"), ".env")

	return paths
}

// projectRoot finds the module/project root by searching upward for a go.mod
// file, starting from the working directory and then the executable's
// directory. Returns "" if none is found (e.g. a deployed binary with no source
// tree), in which case callers fall back to CWD-relative paths.
func projectRoot() string {
	if dir, err := os.Getwd(); err == nil {
		if root, ok := searchUpForGoMod(dir); ok {
			return root
		}
	}
	if exe, err := os.Executable(); err == nil {
		if root, ok := searchUpForGoMod(filepath.Dir(exe)); ok {
			return root
		}
	}
	return ""
}

func searchUpForGoMod(dir string) (string, bool) {
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached the filesystem root
			return "", false
		}
		dir = parent
	}
}

func loadEnv() {
	envOnce.Do(func() {
		for _, path := range envCandidates() {
			file, err := os.Open(path)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				// Skip blank lines and comments.
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				// Allow an optional "export " prefix.
				line = strings.TrimPrefix(line, "export ")

				idx := strings.IndexByte(line, '=')
				if idx < 0 {
					continue
				}

				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])

				// Strip a single pair of surrounding quotes, if present.
				if len(val) >= 2 {
					first, last := val[0], val[len(val)-1]
					if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
						val = val[1 : len(val)-1]
					}
				}

				envValues[key] = val
			}

			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: reading %s: %v\n", path, err)
			}

			file.Close()
			break // first existing file wins
		}
	})
}

// Env returns the string value for key, falling back to fallback when the key
// is not set via the OS environment or the .env file.
func Env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	loadEnv()
	if v, ok := envValues[key]; ok {
		return v
	}

	return fallback
}

// EnvInt returns the integer value for key, or fallback if unset/invalid.
func EnvInt(key string, fallback int) int {
	s := Env(key, "")
	if s == "" {
		return fallback
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

// EnvBytes returns the value for key as a byte slice (fallback when unset).
func EnvBytes(key, fallback string) []byte {
	return []byte(Env(key, fallback))
}

// missing collects the keys requested via RequireEnv* that had no value, so the
// application can report them all at once at startup (see ValidateRequired).
var missing []string

// RequireEnv returns the value for a mandatory key. If the key is not provided
// by the OS environment or the .env file, it is recorded as missing (and the
// empty string is returned). Call ValidateRequired at startup to fail fast.
func RequireEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	loadEnv()
	if v, ok := envValues[key]; ok && v != "" {
		return v
	}
	missing = append(missing, key)
	return ""
}

// RequireEnvInt is the mandatory-int counterpart of RequireEnv.
func RequireEnvInt(key string) int {
	s := RequireEnv(key)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		missing = append(missing, key+" (must be an integer)")
		return 0
	}
	return n
}

// RequireEnvBytes is the mandatory byte-slice counterpart of RequireEnv.
func RequireEnvBytes(key string) []byte {
	return []byte(RequireEnv(key))
}

// MissingRequired returns the list of mandatory keys that had no value.
func MissingRequired() []string {
	return missing
}

// ValidateRequired returns a descriptive error if any mandatory value requested
// via RequireEnv* was missing. Call it once at startup and abort on error.
func ValidateRequired() error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"missing required configuration: %s — set them in .env (see .env.example) or the OS environment",
		strings.Join(missing, ", "),
	)
}
