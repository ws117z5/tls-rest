import React, { Component } from "react";
import Request from "@controllers/request";
import { Buffer } from "buffer";

import "./opencv.css";

interface FilterOption {
  id: string;
  name: string;
  description: string;
}

interface OpenCVProps {}

interface OpenCVState {
  logs: string[];
  isStreaming: boolean;
  streamID: string | null;
  filters: FilterOption[];
  selectedFilter: string;
}

class OpenCV extends Component<OpenCVProps, OpenCVState> {
  private pc!: RTCPeerConnection;
  private localStream: MediaStream | null = null;
  private remoteStream: MediaStream = new MediaStream();

  constructor(props: OpenCVProps) {
    super(props);

    this.state = {
      logs: [],
      isStreaming: false,
      streamID: null,
      filters: [],
      selectedFilter: "motion",
    };
  }

  componentDidMount() {
    // 1. Fetch available OpenCV filters from backend
    Request.apiCall("opencv/filters", {})
      .then((res: { data: FilterOption[] }) => {
        if (res.data && res.data.length > 0) {
          this.setState({
            filters: res.data,
            selectedFilter: res.data[0].id,
          });
          this.log(`Fetched ${res.data.length} filters from server.`);
        }
      })
      .catch((err) => this.log(`Error fetching filters: ${err}`));

    // 2. Initialize WebRTC Peer Connection
    this.pc = new RTCPeerConnection({
      iceServers: [{ urls: "stun:stun.l.google.com:19302" }],
    });

    this.pc.oniceconnectionstatechange = () => {
      this.log(`ICE Connection State: ${this.pc.iceConnectionState}`);
      if (this.pc.iceConnectionState === "connected") {
        this.setState({ isStreaming: true });
      } else if (
        this.pc.iceConnectionState === "failed" ||
        this.pc.iceConnectionState === "disconnected"
      ) {
        this.setState({ isStreaming: false });
      }
    };

    // 3. Receive the annotated OpenCV WebRTC track from Go backend
    this.pc.ontrack = (event: RTCTrackEvent) => {
      this.log(`Received remote track: ${event.track.kind}`);

      event.streams[0]?.getTracks().forEach((track) => {
        this.remoteStream.addTrack(track);
      });

      const video2El = document.getElementById("video2") as HTMLVideoElement | null;
      if (video2El) {
        video2El.srcObject = this.remoteStream;
        video2El.play().catch((err) => this.log(`Remote video play error: ${err}`));
      }
    };

    // 4. Send Offer to backend after ICE Candidate gathering completes
    this.pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
      if (event.candidate === null && this.pc.localDescription) {
        const localDescriptionString = JSON.stringify(this.pc.localDescription);
        const localDescriptionEncoded = Buffer.from(localDescriptionString).toString("base64");

        this.log("ICE Gathering Complete. Initializing WebRTC session...");

        Request.apiCall("opencv", {
          clientSession: localDescriptionEncoded,
          filter: this.state.selectedFilter,
        })
          .then((res: { data: { serverSession: string; streamId: string } }) => {
            this.setState({ streamID: res.data.streamId });
            this.log(`Session initialized on server. Stream ID: ${res.data.streamId}`);

            if (res.data.serverSession) {
              const remoteSessionString = Buffer.from(
                res.data.serverSession,
                "base64"
              ).toString("utf8");

              const remoteSDP = JSON.parse(remoteSessionString);
              return this.pc.setRemoteDescription(new RTCSessionDescription(remoteSDP));
            }
          })
          .then(() => {
            this.log("Remote Description set successfully. WebRTC stream connected.");
          })
          .catch((err) => this.log(`API Exchange Error: ${err}`));
      }
    };

    // Explicitly set transceiver direction to send webcam and receive processed video
    this.pc.addTransceiver("video", { direction: "sendrecv" });

    // 5. Access webcam and attach local media tracks
    navigator.mediaDevices
      .getUserMedia({ video: { width: 640, height: 480 }, audio: false })
      .then((stream: MediaStream) => {
        this.localStream = stream;
        const video1El = document.getElementById("video1") as HTMLVideoElement | null;
        if (video1El) {
          video1El.srcObject = stream;
        }

        stream.getTracks().forEach((track: MediaStreamTrack) => {
          this.pc.addTrack(track, stream);
        });

        return this.pc.createOffer();
      })
      .then((offer) => this.pc.setLocalDescription(offer))
      .then(() => {
        const localEl = document.getElementById(
          "localSessionDescription"
        ) as HTMLTextAreaElement | null;

        if (localEl && this.pc.localDescription) {
          localEl.value = Buffer.from(JSON.stringify(this.pc.localDescription)).toString("base64");
        }
      })
      .catch((err) => this.log(`Media Device Error: ${String(err)}`));
  }

  componentWillUnmount() {
    this.stopMedia();
  }

  stopMedia = (): void => {
    if (this.localStream) {
      this.localStream.getTracks().forEach((track) => track.stop());
      this.localStream = null;
    }

    if (this.remoteStream) {
      this.remoteStream.getTracks().forEach((track) => track.stop());
    }

    const video1El = document.getElementById("video1") as HTMLVideoElement | null;
    if (video1El) video1El.srcObject = null;

    const video2El = document.getElementById("video2") as HTMLVideoElement | null;
    if (video2El) video2El.srcObject = null;

    if (this.pc) {
      this.pc.close();
    }
  };

  handleFilterChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newFilter = e.target.value;
    this.setState({ selectedFilter: newFilter });

    // Mid-stream filter switch API call
    if (this.state.isStreaming && this.state.streamID) {
      Request.apiCall("opencv/filter", {
        streamId: this.state.streamID,
        filter: newFilter,
      })
        .then(() => {
          this.log(`Filter switched mid-stream to: ${newFilter}`);
        })
        .catch((err) => this.log(`Filter switch error: ${err}`));
    }
  };

  log = (msg: string): void => {
    this.setState((prevState) => ({
      logs: [...prevState.logs, `[${new Date().toLocaleTimeString()}] ${msg}`],
    }));
  };

  render() {
    return (
      <div className="opencv-container">
        <h3>WebRTC Hardware Stream-In / Stream-Out Pipeline</h3>

        {/* Filter Selection Dropdown */}
        <div className="filter-selector" style={{ marginBottom: "15px" }}>
          <label htmlFor="filter-select">
            <strong>Select OpenCV Processing Filter: </strong>
          </label>
          <select
            id="filter-select"
            value={this.state.selectedFilter}
            onChange={this.handleFilterChange}
            style={{ padding: "6px 12px", borderRadius: "4px", marginLeft: "8px" }}
          >
            {this.state.filters.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name} — {f.description}
              </option>
            ))}
          </select>
        </div>

        <div className="session-controls">
          <div>
            <label>Browser Base64 Session Offer:</label>
            <br />
            <textarea
              id="localSessionDescription"
              readOnly={true}
              rows={4}
              cols={50}
            ></textarea>
          </div>

          <div>
            <label>Golang Server SDP Status:</label>
            <br />
            <textarea
              id="remoteSessionDescription"
              readOnly={true}
              rows={4}
              cols={50}
              value={
                this.state.isStreaming
                  ? `STREAM ACTIVE (ID: ${this.state.streamID})`
                  : "Establishing WebRTC handshake..."
              }
            ></textarea>
          </div>
        </div>

        <hr />

        <div className="video-streams">
          <div>
            <h4>Local Camera Feed (Input)</h4>
            <video id="video1" width="320" height="240" autoPlay muted playsInline></video>
          </div>

          <div>
            <h4>Processed OpenCV Stream (WebRTC Output)</h4>
            <video
              id="video2"
              width="320"
              height="240"
              autoPlay
              muted
              playsInline
              style={{ display: this.state.isStreaming ? "block" : "none" }}
            ></video>

            {!this.state.isStreaming && (
              <div className="placeholder">Awaiting Connection / Handshake...</div>
            )}
          </div>
        </div>

        <hr />

        <h4>Logs</h4>
        <div id="logs" className="log-window">
          {this.state.logs.map((log, index) => (
            <div key={index} className="log-message">
              {log}
            </div>
          ))}
        </div>
      </div>
    );
  }
}

export default OpenCV;