// Reads image uuids (one per line) on stdin and prints "uuid<TAB>hash" lines.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"tls-rest/go/engine/modules/images/imagehash"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if u := strings.TrimSpace(sc.Text()); u != "" {
			fmt.Printf("%s\t%s\n", u, imagehash.Of(u))
		}
	}
}
