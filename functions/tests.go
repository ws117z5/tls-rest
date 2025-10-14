package functions

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// "/Users/ws117z5/Downloads/Spindump.txt", "/Users/ws117z5/Downloads/Spindump_appsonly.txt"
func fileReadWriteLineByLine(fInName, fOutName string) {
	file, err := os.Open(fInName)

	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	file_write, err := os.Create(fOutName)
	if err != nil {
		fmt.Println(err)
	}
	defer file_write.Close()

	w := bufio.NewWriter(file_write)

	//var lines []string
	scanner := bufio.NewScanner(file)

	//line := ""
	for scanner.Scan() {
		lineTxt := scanner.Text()

		for _, substr := range []string{"Process"} { //, "Path", "Identifier", "Version", "Project Name", "Parent"} {
			i := strings.Index(lineTxt, substr)

			if i == 0 {
				trimmed := strings.TrimSpace(lineTxt[len(substr)+1:])

				fmt.Fprintln(w, trimmed)
				// line += trimmed
				// line += "\t"

				//if substr == "Parent" {
				fmt.Fprintln(w, "")
				//}

				// if substr == "Parent" {
				// 	fmt.Fprintln(w, line)
				// }
			}
		}

		//lines = append(lines, scanner.Text())
	}

	w.Flush()
}
