import Log from "@engine/controllers/log";
import Request from "@engine/controllers/request";

class WebRtc {
    static iceServers: RTCIceServer[] = [
        { urls: "stun:stun.l.google.com:19302" }
    ];

    pc: RTCPeerConnection;
    uuid: string;

    constructor(uuid: string = "") {
        this.uuid = uuid;
        this.pc = new RTCPeerConnection({ iceServers: WebRtc.iceServers });
    }

    init = () => {
        this.pc.oniceconnectionstatechange = (e: Event) => {
            Log.log(this.pc.iceConnectionState);
            Log.log(e);
        };

        this.pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
            if (event.candidate === null) {
                const postParams = {
                    client: JSON.stringify(this.pc.localDescription)
                };
                Request.apiCall(`papers/register/${this.uuid}`, postParams)
                    .then((res: any) => {
                        this.pc.setRemoteDescription(new RTCSessionDescription(res.data));
                    })
                    .catch(Log.log);
            }
        };

        this.pc.onnegotiationneeded = (_e: Event) => {
            this.pc.createOffer()
                .then((d) => this.pc.setLocalDescription(d))
                .catch(Log.log);
        };
    };
}

export default WebRtc;