package functions

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

type Data struct {
	Fieldset map[string]string
	Data     []interface{}
}
type Timestamp time.Time

func GoID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := bytes.Fields(buf[:n])[1]
	id, _ := strconv.ParseInt(string(idField), 10, 64)
	return id
}

// GetRandomHash
func GetRandomHash(n int) (string, error) {
	data := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// MarshalJSON no idea
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	ts := time.Time(*t).Unix()
	stamp := fmt.Sprint(ts)
	return []byte(stamp), nil
}

// UnmarshalJSON no idea
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ts, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*t = Timestamp(time.Unix(int64(ts), 0))
	return nil
}

func GetFields(obj interface{}) map[string]string {
	strct := make(map[string]string)
	st := reflect.TypeOf(obj)

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		if field.Tag.Get("json") != "-" {
			strct[field.Tag.Get("json")] = field.Type.String()
		}
	}

	return strct
}

func ParseJSON(jsonData []byte, data interface{}) (interface{}, error) {

	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		//http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}

	return data, nil
}

func WrapJSON(data interface{}) (string, error) {
	response, err := json.Marshal(data)
	if err != nil {
		return err.Error(), err
	}
	return string(response), nil
}

func ParseRequestBody(requestBody io.ReadCloser, data interface{}) (interface{}, error) {
	//todo if works, remove unmarshal part and write into db straight away
	b, err := io.ReadAll(requestBody)
	//defer r.Body.Close()
	if err != nil {
		//http.Error(w, err.Error(), 500)
		return nil, err
	}

	return ParseJSON(b, data)
}

func SendJSONResponse(w io.Writer, data interface{}) error {
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		//log.Panic(err)
		fmt.Fprintln(w, "error JSON encoding the data response object")
		return err
	}

	return nil
}

// WriteJSON writes v as a JSON response with the given status code. This is the
// single JSON-response helper for HTTP handlers — do not redefine per package.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintln(w, "error JSON encoding the data response object")
	}
}

// JSONError writes {"error": msg} with the given status code.
func JSONError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}
