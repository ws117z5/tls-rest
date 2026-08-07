import React, { Component } from "react";
import Request from "@controllers/request";
import { Buffer } from "buffer";

import "./opencv.css";

interface OpenCVProps {}

interface OpenCVState {
  logs: string[];
}

class OpenCV extends Component<OpenCVProps, OpenCVState> {
  private pc: RTCPeerConnection;

  constructor(props: OpenCVProps) {
    super(props);

    this.state = {
      logs: []
    };

    this.pc = new RTCPeerConnection({
      iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
    });

    this.pc.oniceconnectionstatechange = () => {
      this.log(this.pc.iceConnectionState);
    };

    this.pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
      if (event.candidate === null && this.pc.localDescription) {
        const localSessionDescriptionEl = document.getElementById(
          "localSessionDescription"
        ) as HTMLTextAreaElement | null;

        if (localSessionDescriptionEl) {
          localSessionDescriptionEl.value = btoa(
            JSON.stringify(this.pc.localDescription)
          );
        }

        const localDescriptionString = JSON.stringify(this.pc.localDescription);
        const localDescriptionEncoded = Buffer.from(
          localDescriptionString
        ).toString("base64");

        Request.apiCall("opencv", { clientSession: localDescriptionEncoded }).then(
          (res: { data: { serverSession: string } }) => {
            const remoteSessionDescriptionEl = document.getElementById(
              "remoteSessionDescription"
            ) as HTMLTextAreaElement | null;

            if (remoteSessionDescriptionEl) {
              remoteSessionDescriptionEl.value = res.data.serverSession;
            }
          }
        );
      }
    };

    this.pc.addTransceiver("video", { direction: "sendrecv" });
    this.pc.addTransceiver("audio", { direction: "sendrecv" });

    this.pc
      .createOffer()
      .then((d) => this.pc.setLocalDescription(d))
      .catch((err) => this.log(String(err)));

    this.pc.ontrack = (event: RTCTrackEvent) => {
      const el = document.createElement(
        event.track.kind
      ) as HTMLVideoElement | HTMLAudioElement;
      el.srcObject = event.streams[0];
      el.autoplay = true;
      el.controls = true;

      const remoteVideosEl = document.getElementById("remoteVideos");
      if (remoteVideosEl) {
        remoteVideosEl.appendChild(el);
      }
    };
  }

  componentDidMount() {
    navigator.mediaDevices
      .getUserMedia({ video: true, audio: false })
      .then((stream: MediaStream) => {
        const video1El = document.getElementById(
          "video1"
        ) as HTMLVideoElement | null;

        if (video1El) {
          video1El.srcObject = stream;
        }

        stream.getTracks().forEach((track: MediaStreamTrack) => {
          this.pc.addTrack(track, stream);
        });

        this.pc
          .createOffer()
          .then((d) => this.pc.setLocalDescription(d))
          .catch((err) => this.log(String(err)));
      })
      .catch((err) => this.log(String(err)));
  }

  log = (msg: string): void => {
    this.setState((prevState) => ({
      logs: [...prevState.logs, msg]
    }));
  };

  startSession = (): void => {
    const remoteEl = document.getElementById(
      "remoteSessionDescription"
    ) as HTMLTextAreaElement | null;

    const sd = remoteEl ? remoteEl.value : "";
    if (sd === "") {
      return alert("Session Description must not be empty");
    }

    const buff = Buffer.from(sd, "base64");
    const remoteSession = buff.toString("utf8");

    try {
      this.pc.setRemoteDescription(
        new RTCSessionDescription(JSON.parse(remoteSession))
      );
    } catch (e) {
      alert(e);
    }
  };

  render() {
    return (
      <div>
        Browser base64 Session Description
        <br />
        <textarea id="localSessionDescription" readOnly={true}></textarea>
        <br />

        Golang base64 Session Description
        <br />
        <textarea id="remoteSessionDescription"></textarea>
        <br />
        <button onClick={this.startSession}> Start Session </button>
        <br />
        <br />

        Video
        <br />
        <video id="video1" width="320" height="240" autoPlay muted></video>
        <br />

        Video CV
        <br />
        <div id="remoteVideos"></div>
        <br />

        Logs
        <br />
        <div id="logs">
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