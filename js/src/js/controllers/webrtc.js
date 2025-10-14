class WebRtc {
    static iceServers = [
        {urls: "stun:stun.l.google.com:19382?transport=udp"}
    ];

    pc;

    constructor() {
        this.pc = new RTCPeerConnection(WebRtc.iceServers);
    }

    init = () => {
        this.pc.oniceconnectionstatechange = e => {
            Log.log(_state.pc.iceConnectionState)
            Log.log(e)
        }

        this.pc.onicecandidate = event => {
            if (event.candidate === null) {

                //_state.localSession = _state.pc.localDescription
                let postParams = {
                    client: JSON.stringify(_state.pc.localDescription)
                }
                Request.apiCall(`papers/register/${this.stat.uuid}`, postParams).then((res) => {
                    _state.dc.setRemoteDescription(new RTCSessionDescription(res.Data))
                }).catch(Log.log)
                //Log.log(_state.pc.localDescription)
                //update parent
                //this.props.onUserInit(user.name, user.word, user.uuid, _state.pc.localDescription)
            }
        }

        _state.dc.onnegotiationneeded = e => {
            pc.createOffer().then(d => _state.dc.setLocalDescription(d)).catch(Log.log)
        }
    }
}