package papers

import (
	"fmt"
	"time"

	"tls-rest/go/engine/controllers/functions"

	"github.com/pion/webrtc/v4"
)

func NewMessage(msg, userId, roomId string) *Message {
	return &Message{
		userId,
		roomId,
		time.Now(),
		msg,
	}
}

func (neg *Negotiatior) SpawnPeerConnections() {
	neg.mx.RLock()
	defer neg.mx.RUnlock()
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	for i := len(neg.connectionPool); i > 0; i-- {
		pc, err := webrtc.NewPeerConnection(config)
		if err != nil {
			panic(err)
		}

		// Set the handler for Peer connection state
		// This will notify you when the peer has connected/disconnected
		pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
			fmt.Printf("Peer Connection State has changed: %s\n", s.String())

			if s == webrtc.PeerConnectionStateFailed {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				fmt.Println("Peer Connection has gone to failed exiting")
				//os.Exit(0)

				if cErr := pc.Close(); cErr != nil {
					fmt.Printf("cannot close peerConnection: %v\n", cErr)
				}
			}
		})

		// Register data channel creation handling
		pc.OnDataChannel(func(d *webrtc.DataChannel) {
			fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

			// Register channel opening handling
			d.OnOpen(func() {
				//fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

				// for range time.NewTicker(5 * time.Second).C {
				// 	message := signal.RandSeq(15)
				// 	fmt.Printf("Sending '%s'\n", message)

				// 	// Send the message as text
				// 	sendErr := d.SendText(message)
				// 	if sendErr != nil {
				// 		panic(sendErr)
				// 	}
				// }
			})

			// Register text message handling
			d.OnMessage(func(msg webrtc.DataChannelMessage) {

				//when new connection inited
				var message = new(Message)

				functions.ParseJSON(msg.Data, &message)

				_, userOk := neg.GetUserChannel(message.userId)
				_, roomOk := neg.GetRoomMembers(message.roomId)

				if !roomOk || !userOk {
					//neg.mx.Lock()
					if !userOk {
						neg.PutUserChannel(message.userId, d)
					}

					if !roomOk {
						neg.PutRoomMembers(message.roomId, message.userId)
						//neg.roomUsers[message.roomId] = append(neg.roomUsers[message.roomId], message.userId)
					}

					neg.PutRoomMessage(message.roomId, message.userId, message)
					//neg.roomMessages[message.roomId] = append(neg.roomMessages[message.roomId], *message)
					//d.OnMessage()
				} else {
					neg.notifyAll(message.roomId, *message)
				}

				fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
			})
		})

		neg.connectionPool <- pc
	}
}

// We are forced to call the constructor to get an instance of candidate
func NewNegotiator() *Negotiatior {

	neg := &Negotiatior{
		connectionPool:  make(chan *webrtc.PeerConnection, 2),
		roomMessages:    map[string][]Message{},
		roomUsers:       map[string][]string{},
		userChannels:    map[string]*webrtc.DataChannel{},
		userConnections: map[string]*webrtc.PeerConnection{},
	}

	neg.SpawnPeerConnections()
	return neg
}

func (neg *Negotiatior) notifyAll(room string, message Message) {
	if room, roomExists := neg.roomUsers[room]; roomExists {
		for _, user := range room {
			if channel, channelExists := neg.userChannels[user]; channelExists {
				if message, err := functions.WrapJSON(message); err == nil {
					channel.SendText(message)
				}
			}
		}
	}
}

func (c *Negotiatior) GetRoomMessages(roomId string) ([]Message, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	roomMessages, exists := c.roomMessages[roomId]
	return roomMessages, exists
}

func (c *Negotiatior) PutRoomMessage(roomId string, userId string, msg interface{}) {
	c.mx.Lock()

	switch v := msg.(type) {
	case []byte:
		_msg := NewMessage(string(msg.([]byte)), userId, roomId)
		c.roomMessages[roomId] = append(c.roomMessages[roomId], *_msg)
	case Message:
		c.roomMessages[roomId] = append(c.roomMessages[roomId], msg.(Message))
	default:
		fmt.Printf("Print() invoked with unsupported type: '%T' (expected '%T')\n", msg, v)
		return
	}

	c.mx.Unlock()
}

func (c *Negotiatior) GetUserConnection(userId string) (*webrtc.PeerConnection, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	userConnection, exists := c.userConnections[userId]
	return userConnection, exists
}

func (c *Negotiatior) PutUserConnection(userId string, pc *webrtc.PeerConnection) {
	c.mx.Lock()
	c.userConnections[userId] = pc
	c.mx.Unlock()
}

func (c *Negotiatior) GetUserChannel(userId string) (*webrtc.DataChannel, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	userChannel, exists := c.userChannels[userId]
	return userChannel, exists
}

func (c *Negotiatior) PutUserChannel(userId string, d *webrtc.DataChannel) {
	c.mx.Lock()
	c.userChannels[userId] = d
	c.mx.Unlock()
}

func (c *Negotiatior) GetRoomMembers(roomId string) ([]string, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	roomMembers, exists := c.roomUsers[roomId]
	return roomMembers, exists
}

func (c *Negotiatior) PutRoomMembers(roomId string, userId string) {
	c.mx.Lock()
	if roomMembers, exists := c.roomUsers[roomId]; exists {
		found := false
		for _, user := range roomMembers {
			if user == userId {
				found = true
			}
		}

		if !found {
			c.roomUsers[roomId] = append(c.roomUsers[roomId], userId)
		}
	}

	c.mx.Unlock()
}
