package log

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var log Log

type Log struct {
	logs chan string
}

func New() Log {
	logs := make(chan string, 100)
	return Log{
		logs,
	}
}

func (l *Log) Printf(format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)

	l.logs <- str

	fmt.Println(<-l.logs)

}

func Log1(w http.ResponseWriter, r *http.Request) {
	var request = make(map[string]string)

	b, err := ioutil.ReadAll(r.Body)
	//defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = json.Unmarshal(b, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//todo write to db

}
