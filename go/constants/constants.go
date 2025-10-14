package constants

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// ConfigType extending functions of a basic map container
type ConfigType map[string]interface{}

// AdditionalRights self explanitory
type AdditionalRights struct {
	Edit   string
	Create string
	Delete string
}

// ModuleParams config params of a module
type ModuleParams struct {
	Name             string
	RightsMask       string
	AdditionalRights AdditionalRights
}

var (
	//JsHeader *TODO move to bson config file most of variables, here define structures
	JsHeader = []string{} //"/js/dist/platform.js"}

	//JsHeaderAttr = [][]string{"/js/dist/platform.js"}

	//JsFooter js footer array todo
	JsFooter = []string{"/js/dist/main.js", "/js/dist/gl-matrix-min.js"}

	//Css styles array
	Css = []string{"/css/index.css"}

	//Img Images array
	Img = []string{
		"/img/background.jpg",
		"/img/pano.jpg",
	}

	SQLPath       = "sql"
	SQLBackupPath = "backup"

	//MDb mongodb://foo:bar@localhost:27017
	MDb = Db{
		Addr:     "mongodb://localhost:27017",
		Timeout:  10,
		Database: "tls-rest",
	}

	//PDb TODO move everything related to passwords into another file excluded from git
	PDb = Db{
		Addr:     PDbAddr, //in private.go
		Timeout:  10,
		Database: "tls-rest",
		Password: PdbPass,
		User:     PdbUser,
	}

	//RDb TODO move
	RDb = Db{
		Addr: "localhost:6379",
	}

	//LocalURL TODO move
	LocalURL = "https://localhost"

	//Config a config file
	Config = new(ConfigType)
	/*
		type AType int
		const AuthType (
			Google 		AType = 0
			Facebook 	AType = 1
			VK 			AType = 2
		)
	*/
)

// GetModule returns a module configuration
func (obj *ConfigType) GetModule(name string) (ModuleParams, error) {
	modules := (*obj)["modules"].(map[string]interface{})

	for _, m := range modules {
		module := m.(ModuleParams)

		if module.Name == name {
			return module, nil
		}
	}

	return ModuleParams{}, errors.New("module was not found")

}

// GetParam returns configuration parameter
func (obj *ConfigType) GetParam(param string) string {
	return (*obj)[param].(string)
}

func makeStruct(str string) map[string]interface{} {
	byt := []byte(str)
	var dat map[string]interface{}

	if err := json.Unmarshal(byt, &dat); err != nil {
		panic(err)
	}
	return dat
}

func main() {

	//includeFiles = []string{"/js/react/react.development.js", "/js/react/react-dom.development.js", "/js/app.jsx"}
}

func init() {
	jsonFile, err := os.Open("go.config.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	byteValue, _ := io.ReadAll(jsonFile)

	json.Unmarshal([]byte(byteValue), &Config)

	//fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

}

/*	Rights
	module rights
		1 login

		1 bio

		1 donate

			posts

		1 list
		1 view
		0 edit
		0 create
		0 delete

			comments
		1 list
		1 view
		0 edit
		0 create
		0 delete

			users
		0 list
		0 view
		0 edit
		0 create
		0 delete

		000000001100011111
*/
