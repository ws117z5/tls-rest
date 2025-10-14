package opencv

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/ws117z5/tls-rest/go/controllers/opencv/signal"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfwriter"

	//"github.com/pion/webrtc/v3/pkg/media/oggreader"

	"gocv.io/x/gocv"
)

const (
	frameX      = 960
	frameY      = 720
	frameSize   = frameX * frameY * 3
	minimumArea = 3000
)

type Payload struct {
	ClientSession string `json:"clientSession"`
	ServerSession string `json:"serverSession"`
}

func Init(w http.ResponseWriter, r *http.Request) {
	var payload Payload

	b, err := ioutil.ReadAll(r.Body)
	//defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = json.Unmarshal(b, &payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if payload.ClientSession == "" {
		panic("clientSession is empty")
	}

	ffmpeg := exec.Command("ffmpeg", "-i", "pipe:0", "-pix_fmt", "bgr24", "-s", strconv.Itoa(frameX)+"x"+strconv.Itoa(frameY), "-f", "rawvideo", "pipe:1") //nolint
	ffmpegIn, _ := ffmpeg.StdinPipe()
	ffmpegOut, _ := ffmpeg.StdoutPipe()
	ffmpegErr, _ := ffmpeg.StderrPipe()

	if err := ffmpeg.Start(); err != nil {
		panic(err)
	}

	go func() {
		scanner := bufio.NewScanner(ffmpegErr)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	peerConnection, err := createWebRTCConn(ffmpegIn, w, payload.ClientSession)

	if err != nil {
		panic(err)
	}
	go startGoCVMotionDetect(ffmpegOut, peerConnection)
}

// func main() {
// 	ffmpeg := exec.Command("ffmpeg", "-i", "pipe:0", "-pix_fmt", "bgr24", "-s", strconv.Itoa(frameX)+"x"+strconv.Itoa(frameY), "-f", "rawvideo", "pipe:1") //nolint
// 	ffmpegIn, _ := ffmpeg.StdinPipe()
// 	ffmpegOut, _ := ffmpeg.StdoutPipe()
// 	ffmpegErr, _ := ffmpeg.StderrPipe()

// 	if err := ffmpeg.Start(); err != nil {
// 		panic(err)
// 	}

// 	go func() {
// 		scanner := bufio.NewScanner(ffmpegErr)
// 		for scanner.Scan() {
// 			fmt.Println(scanner.Text())
// 		}
// 	}()

// 	createWebRTCConn(ffmpegIn)
// 	startGoCVMotionDetect(ffmpegOut)
// }

// This was taken from the GoCV examples, the only change is we are taking a buffer from ffmpeg instead of webcam
// https://github.com/hybridgroup/gocv/blob/master/cmd/motion-detect/main.go
func startGoCVMotionDetect(ffmpegOut io.Reader, peerConnection *webrtc.PeerConnection) {
	//window := gocv.NewWindow("Motion Window")
	//defer window.Close() //nolint

	//window := GetWindow()

	img := gocv.NewMat()
	defer img.Close() //nolint

	imgDelta := gocv.NewMat()
	defer imgDelta.Close() //nolint

	imgThresh := gocv.NewMat()
	defer imgThresh.Close() //nolint

	mog2 := gocv.NewBackgroundSubtractorMOG2()
	defer mog2.Close() //nolint

	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	rtpSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	for {
		buf := make([]byte, frameSize)
		if _, err := io.ReadFull(ffmpegOut, buf); err != nil {
			fmt.Println(err)
			continue
		}
		img, _ := gocv.NewMatFromBytes(frameY, frameX, gocv.MatTypeCV8UC3, buf)
		if img.Empty() {
			continue
		}

		status := "Ready"
		statusColor := color.RGBA{0, 255, 0, 0}

		// first phase of cleaning up image, obtain foreground only
		mog2.Apply(img, &imgDelta)

		// remaining cleanup of the image to use for finding contours.
		// first use threshold
		gocv.Threshold(imgDelta, &imgThresh, 25, 255, gocv.ThresholdBinary)

		// then dilate
		kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
		defer kernel.Close() //nolint
		gocv.Dilate(imgThresh, &imgThresh, kernel)

		// now find contours
		contours := gocv.FindContours(imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)

		for i := 0; i < contours.Size(); i++ {
			area := gocv.ContourArea(contours.At(i))
			if area < minimumArea {
				continue
			}

			status = "Motion detected"
			statusColor = color.RGBA{255, 0, 0, 0}
			gocv.DrawContours(&img, contours, i, statusColor, 2)

			rect := gocv.BoundingRect(contours.At(i))
			gocv.Rectangle(&img, rect, color.RGBA{0, 0, 255, 0}, 2)
		}

		gocv.PutText(&img, status, image.Pt(10, 20), gocv.FontHersheyPlain, 1.2, statusColor, 2)

		if ivfErr := videoTrack.WriteSample(media.Sample{Data: img.ToBytes(), Duration: time.Second / 24}); ivfErr != nil {
			panic(ivfErr)
		}

		// window.IMShow(img)
		// if window.WaitKey(1) == 27 {
		// 	break
		// }
	}

}

func createWebRTCConn(ffmpegIn io.Writer, w http.ResponseWriter, clientSession string) (*webrtc.PeerConnection, error) {
	ivfWriter, err := ivfwriter.NewWith(ffmpegIn)
	if err != nil {
		panic(err)
	}

	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().RTPCodecCapability.MimeType)
		for {
			// Read RTP packets being sent to Pion
			rtp, _, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			if ivfWriterErr := ivfWriter.WriteRTP(rtp); ivfWriterErr != nil {
				panic(ivfWriterErr)
			}
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}

	//recieving from request params instead
	//signal.Decode(signal.MustReadStdin(), &offer)

	signal.Decode(clientSession, &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	localDescription := signal.Encode(*peerConnection.LocalDescription())

	payload := Payload{clientSession, localDescription}

	err = json.NewEncoder(w).Encode(&payload)
	if err != nil {
		log.Panic(err)
		fmt.Fprintln(w, "OpenCV Encoder Error!")
	}
	//fmt.Println(localDescription)

	return peerConnection, err
}
