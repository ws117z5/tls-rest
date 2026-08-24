package papers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"tls-rest/go/engine/controllers/functions"
	"tls-rest/go/engine/controllers/module"

	"tls-rest/go/engine/controllers/db/pgdb"

	"github.com/go-pg/urlstruct"
	"github.com/gorilla/mux"
)

// proomColumns is the SELECT list for the paper-room table(s).
const proomColumns = "uuid, id, name, password, created_by, users, created"

// rowToProom maps a result-set row (from RQuery) into a Proom.
func rowToProom(row map[string]interface{}) Proom {
	return Proom{
		Uuid:      pgdb.Coerce[string](row["uuid"]),
		Id:        pgdb.Coerce[int64](row["id"]),
		Name:      pgdb.Coerce[string](row["name"]),
		Password:  pgdb.Coerce[string](row["password"]),
		CreatedBy: pgdb.Coerce[string](row["created_by"]),
		Users:     pgdb.Coerce[string](row["users"]),
		Created:   pgdb.Coerce[time.Time](row["created"]),
	}
}

var neg *Negotiatior

func ExitRoom(w http.ResponseWriter, r *http.Request) {

}

//TODO try moving everithing into mongodb, using uuids as keys and json as a wrapper

func List(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "List Posts!")
	rooms := make([]Proom, 0)
	//var values = pager.Values(r.URL.Query())

	filter := new(Filter)
	ctx := new(context.Context)
	keys := r.URL.Query()
	err := urlstruct.Unmarshal(*ctx, keys, filter)
	if err != nil {
		panic(err)
	}

	db, _ := pgdb.GetInstance()
	rows, err := db.GetAll("SELECT " + proomColumns + " FROM prooms")
	if err != nil {
		log.Println(err)
	}
	for _, row := range rows {
		rooms = append(rooms, rowToProom(row))
	}

	//todo clean inactive rooms

	functions.SendJSONResponse(w, Data{functions.GetFields(Proom{}), rooms})
}

func ViewRoom(w http.ResponseWriter, r *http.Request) {
	//vars := mux.Vars(r)
	//vals := r.URL.Query()
}

func CreateRoom(w http.ResponseWriter, r *http.Request) {
	//vars := mux.Vars(r)
	//vals := r.URL.Query()

	var pRoom Proom

	b, err := io.ReadAll(r.Body)
	//defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = json.Unmarshal(b, &pRoom)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db, _ := pgdb.GetInstance()
	_, err = db.InsertRow("prooms", map[string]interface{}{
		"uuid":       pRoom.Uuid,
		"name":       pRoom.Name,
		"password":   pRoom.Password,
		"created_by": pRoom.CreatedBy,
		"users":      pRoom.Users,
	})

	if err != nil {
		// A failed insert is a server/DB problem, not a client "bad request".
		// Log the underlying error (missing table, NOT NULL/constraint, etc.)
		// so the actual cause is visible rather than a bare 400 in the browser.
		log.Printf("papers CreateRoom: insert failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make(map[string]string)
	resp["message"] = "Room Created"
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)

}

func RegisterUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userId := vars["userId"]
	roomId := vars["roomId"]

	//var dc *webrtc.PeerConnection

	dc := <-neg.GetConnections()
	defer neg.SpawnPeerConnections()
	defer neg.PutUserConnection(userId, dc)
	defer neg.PutRoomMembers(roomId, userId)

	params := make(map[string]string)

	//todo if works, remove unmarshal part and write into db straight away
	b, err := io.ReadAll(r.Body)
	//defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = json.Unmarshal(b, &params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

}

func AddRoomUser(w http.ResponseWriter, r *http.Request) {
	var newUser Puser
	var currentUsers []Puser
	//var pRoom PRoom

	vars := mux.Vars(r)
	roomId := vars["roomId"]

	//todo if works, remove unmarshal part and write into db straight away
	b, err := io.ReadAll(r.Body)
	//defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = json.Unmarshal(b, &newUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db, _ := pgdb.GetInstance()
	pRoom := new(Proom)

	prow, err := db.GetOne("SELECT "+proomColumns+" FROM proom WHERE uuid = $1", roomId)
	if err != nil {
		panic(err)
	}
	if prow == nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	*pRoom = rowToProom(prow)

	err = json.Unmarshal([]byte(pRoom.Users), &currentUsers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated := false
	for _, user := range currentUsers {
		if user.Uuid == newUser.Uuid {
			user = newUser
			updated = true
		}
	}
	if !updated {
		currentUsers = append(currentUsers, newUser)
	}

	jsonUsers, err := json.Marshal(currentUsers)
	if err != nil {
		fmt.Println("error:", err)
	}

	pRoom.Users = string(jsonUsers)

	_, err = db.InsertRow("prooms", map[string]interface{}{
		"uuid":       pRoom.Uuid,
		"name":       pRoom.Name,
		"password":   pRoom.Password,
		"created_by": pRoom.CreatedBy,
		"users":      pRoom.Users,
	})

	if err != nil {
		panic(err)
	}

	w.Write(jsonUsers)
}

func ViewRoomUsers(w http.ResponseWriter, r *http.Request) {
	//var currentUsers []PUser
	//var pRoom PRoom

	vars := mux.Vars(r)
	roomId := vars["roomId"]

	db, _ := pgdb.GetInstance()
	pRoom := new(Proom)

	prow, err := db.GetOne("SELECT "+proomColumns+" FROM proom WHERE uuid = $1", roomId)
	if err != nil {
		panic(err)
	}
	if prow == nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	*pRoom = rowToProom(prow)

	jsonUsers, err := json.Marshal(pRoom.Users)
	if err != nil {
		fmt.Println("error:", err)
	}

	w.Write(jsonUsers)
	//fmt.Fprintln(w, string(jsonUsers))
}

var Page = &module.PageAbstract{
	ID:            "papers",
	Name:          "Papers Game ",
	Submenu:       "games",
	RequiresAuth:  true,
	RequiresAdmin: true,
	Routes: []module.PageRoute{
		{Path: "/papers", Methods: []string{http.MethodGet}, Handler: List},
		{Path: "/papers/create", Methods: []string{http.MethodPost}, Handler: CreateRoom},
		{Path: "/papers/{roomId}", Methods: []string{http.MethodPost}, Handler: AddRoomUser},
		{Path: "/papers/{roomId}", Methods: []string{http.MethodGet}, Handler: ViewRoomUsers},
		{Path: "/papers/{roomId}/report", Methods: []string{http.MethodPost}, Handler: ReportLink},
		{Path: "/papers/{roomId}/plan", Methods: []string{http.MethodGet}, Handler: GetPlan},
		{Path: "/papers/{roomId}/{userId}", Methods: []string{http.MethodGet}, Handler: RegisterUser},
	},
}

func Init() {
	neg = NewNegotiator()
	Page.Initialize()
}
