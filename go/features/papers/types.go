package papers

import (
	"sync"
	"time"
	"tls-rest/go/lib/mesh"

	"github.com/go-pg/urlstruct"
	"github.com/pion/webrtc/v4"
)

// papers page
type Data struct {
	Fieldset map[string]string
	Data     []Proom
}

type Proom struct {
	Uuid      string    `json:"uuid"`
	Id        int64     `json:"id"`
	Name      string    `json:"name"`
	Password  string    `json:"password"`
	CreatedBy string    `json:"created_by"`
	Users     string    `json:"users"`
	Created   time.Time `sql:"default:now()" json:"created"`
}

type Puser struct {
	Uuid    string `json:"uuid"`
	Id      int64  `json:"id"`
	Name    string `json:"name"`
	Session string `json:"session"`
}

type Filter struct {
	tableName struct{} `sql:"prooms" urlstruct:"b"`

	urlstruct.Pager
}

// Negotiator
type connection struct {
	pc *webrtc.PeerConnection
	dc *webrtc.DataChannel
}

type negotiatior struct {
	connectionPool  chan *webrtc.PeerConnection
	roomMessages    map[string][]message
	roomUsers       map[string][]string
	userConnections map[string]*webrtc.PeerConnection
	userChannels    map[string]*webrtc.DataChannel

	mx sync.RWMutex
	//notifyAll func(string)
	//addUser func(string)
	//addRoom func(string, string)
}

//mesh handler

// reportRequest is the body a peer POSTs to /papers/{roomId}/report.
type reportRequest struct {
	Peer  string          `json:"peer"`
	Up    float64         `json:"up"`
	Down  float64         `json:"down"`
	Stats []mesh.LinkStat `json:"stats"`
}
