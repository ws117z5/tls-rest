package functions

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"tls-rest/go/engine/controllers/log"
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

func PostParam[T any](name string, r *http.Request) (T, error) {
	var ret T
	params, ok := r.Context().Value(http.MethodPost).(map[string]interface{})
	if !ok {
		return ret, errors.New("Post param wasn't found")
	}
	raw, exists := params[name]
	if !exists || raw == nil {
		// Missing or explicitly null (e.g. a fresh client sending no cached
		// "hash"): return the zero value, never panic on a nil assertion.
		return ret, errors.New("Post param wasn't found")
	}
	val, ok := raw.(T)
	if !ok {
		return ret, fmt.Errorf("post param %q has unexpected type", name)
	}
	return val, nil
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

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func Int(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case float64:
		return int(n)
	case string:
		val, _ := strconv.Atoi(n)
		return val
	case nil:
		return 0
	default:
		return 0
	}
}

// truthy interprets a checkbox cell value (bool, "true"/"1", 1, etc.).
func Truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "on"
	case float64:
		return t != 0
	case int:
		return t != 0
	}
	return false
}

// --- Type coercion helpers (same as MySQL version) ---

func CoerceByType(data map[string]interface{}, current *map[string]interface{}, schemaType reflect.Type) error {
	for i := 0; i < schemaType.NumField(); i++ {
		field := schemaType.Field(i)
		key := field.Tag.Get("db")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		expectedType := field.Type

		raw, ok := data[key]
		if !ok {
			log.Warningf("Missing value for field %s", key)
			continue
		}

		val, err := CoerceValue(raw, expectedType)
		if err != nil {
			return fmt.Errorf("coercing field %s: %w", key, err)
		}

		(*current)[key] = val
	}

	return nil
}

func CoerceToSchema[T any](data map[string]interface{}) (map[string]interface{}, error) {
	var schema T

	schemaType := reflect.TypeOf(schema)

	coerced := make(map[string]interface{})

	for i := 0; i < schemaType.NumField(); i++ {
		field := schemaType.Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if err := CoerceByType(data, &coerced, field.Type); err != nil {
				return nil, err
			}
			continue
		}

		key := field.Tag.Get("db")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		expectedType := field.Type

		raw, ok := data[key]
		if !ok {
			continue
		}

		val, err := CoerceValue(raw, expectedType)
		if err != nil {
			return nil, fmt.Errorf("coercing field %s: %w", key, err)
		}

		coerced[key] = val
	}

	return coerced, nil
}

func CoerceValue(value interface{}, t reflect.Type) (interface{}, error) {
	// Concrete types not distinguished by Kind alone.
	switch t {
	case reflect.TypeOf([]byte(nil)):
		return Bytes(value), nil
	case reflect.TypeOf(time.Time{}):
		if tm, ok := value.(time.Time); ok {
			return tm, nil
		}
		return time.Time{}, nil
	}

	switch t.Kind() {
	case reflect.String:
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		case nil:
			return "", nil
		case fmt.Stringer:
			return v.String(), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	case reflect.Int:
		return int(Int64(value)), nil
	case reflect.Int32:
		return int32(Int64(value)), nil
	case reflect.Int64:
		return Int64(value), nil
	case reflect.Float64:
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case int:
			return float64(v), nil
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f, nil
		}
		return float64(0), nil
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			b, _ := strconv.ParseBool(v)
			return b, nil
		}
		return false, nil
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return Bytes(value), nil
		}
	}
	return value, nil
}

// Coerce converts a DB result value to T using CoerceValue (the single coercion
// authority). It returns the zero value of T when the value can't be converted,
// and is the ergonomic way to read a map value from RQuery/GetOne/GetAll as a
// concrete Go type, e.g. Coerce[int64](row["id"]) or Coerce[string](row["name"]).
func Coerce[T any](v interface{}) T {
	var zero T
	out, err := CoerceValue(v, reflect.TypeOf(zero))
	if err != nil {
		return zero
	}
	if typed, ok := out.(T); ok {
		return typed
	}
	return zero
}

func Int64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		x, _ := strconv.ParseInt(n, 10, 64)
		return x
	}
	return 0
}

func Bytes(v interface{}) []byte {
	switch b := v.(type) {
	case []byte:
		return b
	case string:
		return []byte(b)
	}
	return nil
}
