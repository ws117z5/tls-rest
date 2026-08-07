import Log from "@controllers/log";
import Request from "@controllers/request";

class WebRtc {
    static iceServers: RTCIceServer[] = [
        {urls: "stun:stun.l.google.com:19302?transport=udp"}
    ];

    pc: RTCPeerConnection;
    dc: RTCDataChannel;
    uuid: string;

    constructor() {
        this.pc = new RTCPeerConnection({ iceServers: WebRtc.iceServers });
    }

    init = () => {
        this.pc.oniceconnectionstatechange = (e: Event) => {
            Log.log(this.pc.iceConnectionState)
            Log.log(e)
        }

        this.pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
            if (event.candidate === null) {

                //_state.localSession = _state.pc.localDescription
                let postParams = {
                    client: JSON.stringify(this.pc.localDescription)
                }
                Request.apiCall(`papers/register/${this.uuid}`, postParams).then((res: any) => {
                    this.dc.setRemoteDescription(new RTCSessionDescription(res.Data))
                }).catch(Log.log)
                //Log.log(_state.pc.localDescription)
                //update parent
                //this.props.onUserInit(user.name, user.word, user.uuid, _state.pc.localDescription)
            }
        }

        this.dc.onnegotiationneeded = e => {
            this.pc.createOffer().then(d => this.dc.setLocalDescription(d)).catch(Log.log)
        }
    }
}