package opencv

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"tls-rest/go/features/opencv/signal"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
	"github.com/pion/webrtc/v4/pkg/media/ivfwriter"
	"gocv.io/x/gocv"

	features "tls-rest/go/features/opencv"
)

const (
	frameX      = 640
	frameY      = 480
	targetFPS   = 30
	frameSize   = frameX * frameY * 3
	minimumArea = 2000
)

type Payload struct {
	ClientSession string `json:"clientSession"`
	ServerSession string `json:"serverSession"`
	StreamID      string `json:"streamId,omitempty"`
	Filter        string `json:"filter,omitempty"`
}

// ------------------------------------------------------------------
// STREAM MANAGER (Thread-Safe Active Stream Sessions)
// ------------------------------------------------------------------

type StreamSession struct {
	sync.RWMutex
	PeerConnection *webrtc.PeerConnection
	ActiveFilter   FrameFilter
	VideoTrack     *webrtc.TrackRemote
}

type StreamManager struct {
	sync.RWMutex
	streams map[string]*StreamSession
}

var Manager = &StreamManager{
	streams: make(map[string]*StreamSession),
}

func (m *StreamManager) Register(streamID string, pc *webrtc.PeerConnection, filterID string) *StreamSession {
	m.Lock()
	defer m.Unlock()

	session := &StreamSession{
		PeerConnection: pc,
		ActiveFilter:   FilterFactory(filterID),
	}
	m.streams[streamID] = session
	log.Printf("[MANAGER] Stream registered: %s (Initial Filter: %s)", streamID, filterID)
	return session
}

func (m *StreamManager) UpdateFilter(streamID string, newFilterID string) error {
	m.RLock()
	session, exists := m.streams[streamID]
	m.RUnlock()

	if !exists {
		return fmt.Errorf("stream %s not found", streamID)
	}

	session.Lock()
	defer session.Unlock()

	if session.ActiveFilter != nil {
		session.ActiveFilter.Close()
	}

	session.ActiveFilter = FilterFactory(newFilterID)
	log.Printf("[MANAGER][%s] Switched active filter to: %s", streamID, newFilterID)

	// Send PLI Keyframe request to browser so new filter initializes on a clean intra-frame
	if session.PeerConnection != nil && session.VideoTrack != nil {
		_ = session.PeerConnection.WriteRTCP([]rtcp.Packet{
			&rtcp.PictureLossIndication{MediaSSRC: uint32(session.VideoTrack.SSRC())},
		})
	}

	return nil
}

func (m *StreamManager) Remove(streamID string) {
	m.Lock()
	defer m.Unlock()
	if session, exists := m.streams[streamID]; exists {
		session.Lock()
		if session.ActiveFilter != nil {
			session.ActiveFilter.Close()
		}
		if session.PeerConnection != nil {
			_ = session.PeerConnection.Close()
		}
		session.Unlock()

		delete(m.streams, streamID)
		log.Printf("[MANAGER] Stream removed: %s", streamID)
	}
}

func generateStreamID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ------------------------------------------------------------------
// WEBRTC & OPENCV PIPELINE INIT
// ------------------------------------------------------------------

func Init(w http.ResponseWriter, r *http.Request) {
	os.Setenv("OPENCV_OPENCL_DEVICE", "GPU")

	var payload Payload
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[INIT] Failed to read request body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = json.Unmarshal(b, &payload); err != nil {
		log.Printf("[INIT] JSON unmarshal failure: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if payload.ClientSession == "" {
		http.Error(w, "clientSession is required", http.StatusBadRequest)
		return
	}

	streamID := payload.StreamID
	if streamID == "" {
		streamID = generateStreamID()
	}

	initialFilter := payload.Filter
	if initialFilter == "" {
		initialFilter = "motion"
	}

	log.Printf("[INIT][%s] Initializing Hardware Stream-In -> Stream-Out Pipeline...", streamID)

	hwProfile := features.DetectHardware()
	log.Printf("[INIT][%s] Detected Hardware Profile: %s", streamID, hwProfile.Name)

	// 1. INPUT FFMPEG (WebRTC VP8/IVF -> Raw BGR Frames)
	inArgs := append([]string{"-loglevel", "error"}, hwProfile.HWAccelIn...)
	inArgs = append(inArgs,
		"-analyzeduration", "1000000",
		"-probesize", "1000000",
		"-fflags", "+nobuffer+discardcorrupt",
		"-i", "pipe:0",
		"-pix_fmt", "bgr24",
		"-s", fmt.Sprintf("%dx%d", frameX, frameY),
		"-f", "rawvideo",
		"pipe:1",
	)
	inFFmpeg := exec.Command("ffmpeg", inArgs...)

	inPipeIn, err := inFFmpeg.StdinPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	inPipeOut, err := inFFmpeg.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	inStderr, _ := inFFmpeg.StderrPipe()
	if err := inFFmpeg.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		scanner := bufio.NewScanner(inStderr)
		for scanner.Scan() {
			log.Printf("[FFMPEG IN STDERR][%s] %s", streamID, scanner.Text())
		}
	}()

	// 2. OUTPUT FFMPEG (Raw BGR Frames -> Encoder -> IVF Pipe)
	outArgs := []string{
		"-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "bgr24",
		"-s", fmt.Sprintf("%dx%d", frameX, frameY),
		"-r", fmt.Sprintf("%d", targetFPS),
		"-i", "pipe:0",
	}
	outArgs = append(outArgs, hwProfile.HWAccelOut...)
	outArgs = append(outArgs,
		"-g", "30", // Force keyframe interval
		"-f", "ivf",
		"pipe:1",
	)
	outFFmpeg := exec.Command("ffmpeg", outArgs...)

	outPipeIn, err := outFFmpeg.StdinPipe()
	if err != nil {
		_ = inFFmpeg.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outPipeOut, err := outFFmpeg.StdoutPipe()
	if err != nil {
		_ = inFFmpeg.Process.Kill()
		_ = outFFmpeg.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outStderr, _ := outFFmpeg.StderrPipe()
	if err := outFFmpeg.Start(); err != nil {
		_ = inFFmpeg.Process.Kill()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		scanner := bufio.NewScanner(outStderr)
		for scanner.Scan() {
			log.Printf("[FFMPEG OUT STDERR][%s] %s", streamID, scanner.Text())
		}
	}()

	// 3. WEBRTC PEER CONNECTION SETUP
	peerConnection, outboundTrack, session, err := createWebRTCConn(inPipeIn, w, payload.ClientSession, streamID, initialFilter)
	if err != nil {
		log.Printf("[WEBRTC][%s] Setup error: %v", streamID, err)
		_ = inFFmpeg.Process.Kill()
		_ = outFFmpeg.Process.Kill()
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}

	_ = peerConnection

	// 4. STREAM OUT GOROUTINE (Read IVF -> Write WebRTC Track)
	go func() {
		ivfReader, header, err := ivfreader.NewWith(outPipeOut)
		if err != nil {
			log.Printf("[IVF READ][%s] Failed initializing IVF reader: %v", streamID, err)
			return
		}

		for {
			frame, _, err := ivfReader.ParseNextFrame()
			if err != nil {
				log.Printf("[IVF READ][%s] Stream closed: %v", streamID, err)
				break
			}

			if writeErr := outboundTrack.WriteSample(media.Sample{
				Data:     frame,
				Duration: time.Second / time.Duration(header.TimebaseDenominator),
			}); writeErr != nil {
				log.Printf("[WEBRTC WRITE][%s] Error writing track sample: %v", streamID, writeErr)
				break
			}
		}
	}()

	// 5. OPENCV WORKER LOOP
	go startGoCVProcessing(inPipeOut, outPipeIn, session, inFFmpeg, outFFmpeg, streamID)
}

func startGoCVProcessing(
	inPipeOut io.Reader,
	outPipeIn io.Writer,
	session *StreamSession,
	inCmd *exec.Cmd,
	outCmd *exec.Cmd,
	streamID string,
) {
	log.Printf("[OPENCV][%s] Starting OpenCV processing worker...", streamID)

	defer func() {
		log.Printf("[OPENCV][%s] Cleaning up stream processes...", streamID)
		if inCmd.Process != nil {
			_ = inCmd.Process.Kill()
		}
		if outCmd.Process != nil {
			_ = outCmd.Process.Kill()
		}
		Manager.Remove(streamID)
	}()

	frameChan := make(chan []byte, 1)

	// Non-blocking reader from input FFmpeg
	go func() {
		for {
			buf := make([]byte, frameSize)
			if _, err := io.ReadFull(inPipeOut, buf); err != nil {
				close(frameChan)
				return
			}

			select {
			case frameChan <- buf:
			default:
				// Non-blocking drop to keep 0-latency sync
			}
		}
	}()

	frameInterval := time.Second / time.Duration(targetFPS)

	for buf := range frameChan {
		startTime := time.Now()

		img, err := gocv.NewMatFromBytes(frameY, frameX, gocv.MatTypeCV8UC3, buf)
		if err != nil || img.Empty() {
			continue
		}

		// Apply currently active filter safely under read-lock
		session.RLock()
		if session.ActiveFilter != nil {
			session.ActiveFilter.Process(&img)
		}
		session.RUnlock()

		if _, err := outPipeIn.Write(img.ToBytes()); err != nil {
			_ = img.Close()
			break
		}

		_ = img.Close()

		elapsed := time.Since(startTime)
		if elapsed < frameInterval {
			time.Sleep(frameInterval - elapsed)
		}
	}
}

func createWebRTCConn(
	ffmpegIn io.Writer,
	w http.ResponseWriter,
	clientSession string,
	streamID string,
	initialFilter string,
) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, *StreamSession, error) {

	ivfWriter, err := ivfwriter.NewWith(ffmpegIn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("IVFWriter failure: %w", err)
	}

	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4, webrtc.NetworkTypeUDP6,
		webrtc.NetworkTypeTCP4, webrtc.NetworkTypeTCP6,
	})

	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("NewPeerConnection failure: %w", err)
	}

	session := Manager.Register(streamID, peerConnection, initialFilter)

	outboundTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video",
		"gocv-stream",
	)
	if err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("failed creating outbound track: %w", err)
	}

	if _, err := peerConnection.AddTrack(outboundTrack); err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("failed adding outbound track: %w", err)
	}

	var trackOnce sync.Once
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if track.Kind() != webrtc.RTPCodecTypeVideo {
			return
		}

		trackOnce.Do(func() {
			log.Printf("[WEBRTC][%s] Incoming Video Track Bound! Codec: %s", streamID, track.Codec().MimeType)

			session.Lock()
			session.VideoTrack = track
			session.Unlock()

			// Periodically request keyframes
			go func() {
				ticker := time.NewTicker(time.Second * 2)
				defer ticker.Stop()
				for range ticker.C {
					_ = peerConnection.WriteRTCP([]rtcp.Packet{
						&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())},
					})
				}
			}()

			hasKeyFrame := false
			for {
				rtpPacket, _, readErr := track.ReadRTP()
				if readErr != nil {
					return
				}

				// Keyframe gatekeeper: skip inter-frames until first VP8 keyframe arrives
				if !hasKeyFrame {
					if isVP8Keyframe(rtpPacket.Payload) {
						hasKeyFrame = true
						log.Printf("[WEBRTC][%s] First VP8 keyframe received!", streamID)
					} else {
						continue
					}
				}

				if ivfWriterErr := ivfWriter.WriteRTP(rtpPacket); ivfWriterErr != nil {
					return
				}
			}
		})
	})

	offer := webrtc.SessionDescription{}
	if err := signal.Decode(clientSession, &offer); err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("failed decoding offer: %w", err)
	}

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("SetRemoteDescription failure: %w", err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("CreateAnswer failure: %w", err)
	}

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("SetLocalDescription failure: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	select {
	case <-gatherComplete:
	case <-time.After(3 * time.Second):
	}

	localDescription := signal.Encode(*peerConnection.LocalDescription())
	payload := Payload{
		ClientSession: clientSession,
		ServerSession: localDescription,
		StreamID:      streamID,
		Filter:        initialFilter,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&payload); err != nil {
		_ = peerConnection.Close()
		return nil, nil, nil, fmt.Errorf("failed encoding payload: %w", err)
	}

	return peerConnection, outboundTrack, session, nil
}

func isVP8Keyframe(payload []byte) bool {
	if len(payload) < 1 {
		return false
	}
	hasX := (payload[0] & 0x80) != 0
	offset := 1

	if hasX && len(payload) > 1 {
		hasI := (payload[1] & 0x80) != 0
		hasL := (payload[1] & 0x40) != 0
		hasTOrK := (payload[1]&0x20) != 0 || (payload[1]&0x10) != 0
		offset++

		if hasI {
			if len(payload) > offset && (payload[offset]&0x80) != 0 {
				offset += 2
			} else {
				offset += 1
			}
		}
		if hasL {
			offset++
		}
		if hasTOrK {
			offset++
		}
	}

	if len(payload) <= offset {
		return false
	}

	isStartOfFrame := (payload[0] & 0x10) != 0
	isKeyFrame := (payload[offset] & 0x01) == 0

	return isStartOfFrame && isKeyFrame
}
