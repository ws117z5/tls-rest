package input

import (
	"bufio"
	"fmt"
	"os"
)

// ReadCommand for realtime debugging
func ReadCommand() {

	for {
		reader := bufio.NewReader(os.Stdin)
		//fmt.Print("Enter text: ")
		text, _ := reader.ReadString('\n')
		fmt.Println(text)
	}
}
