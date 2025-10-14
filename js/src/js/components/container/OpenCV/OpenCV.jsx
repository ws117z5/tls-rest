import React, { Component } from "react";
import Request from "../../../controllers/request";
import { Buffer } from "buffer";

import "./opencv.css";

//todo
class OpenCV extends Component {
    constructor(props, state) {
        super(props, state);

        this.state = {
            logs: []
        }

        var that = this;

        
        this.pc = new RTCPeerConnection({
            iceServers: [ { urls: 'stun:stun.l.google.com:19302'} ]
        })

        this.pc.oniceconnectionstatechange = (e) => {
            //log(pc.iceConnectionState)
            // this.setState(prevState => ({
            //     logs: [...prevState.logs, pc.iceConnectionState]
            // })) 
            that.log(that.pc.iceConnectionState)  
        }

        
        this.pc.onicecandidate = (event) => {
            if (event.candidate === null) {
                document.getElementById('localSessionDescription').value = btoa(JSON.stringify(that.pc.localDescription))

                const localDescriptionString = JSON.stringify(that.pc.localDescription);
                const localDescriptionEncoded = Buffer.from(localDescriptionString).toString('base64');
                const b46 = btoa(JSON.stringify(that.pc.localDescription))

                Request.apiCall('opencv', {clientSession: localDescriptionEncoded}).then((res) => {


                    document.getElementById('remoteSessionDescription').value = res.data.serverSession;
                    // const buff = new Buffer(res.serverSession, 'base64');
                    // const text = buff.toString('utf8');

                    // console.log(text);
                    
              
                    // window.startSession = () => {
                    //     let sd = document.getElementById('remoteSessionDescription').value
                    //     if (sd === '') {
                    //         return alert('Session Description must not be empty')
                    //     }
                      
                    //     try {
                    //         pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
                    //     } catch (e) {
                    //         alert(e)
                    //     }
                    // }
                });
            }
        }

        this.pc.addTransceiver('video', {
            direction: 'sendrecv'
        })
        this.pc.addTransceiver('audio', {
            direction: 'sendrecv'
        })

        this.pc.createOffer()
            .then(d => that.pc.setLocalDescription(d))
            .catch(that.log);

        this.pc.ontrack = function (event) {
            var el = document.createElement(event.track.kind);
            el.srcObject = event.streams[0];
            el.autoplay = true;
            el.controls = true;

            document.getElementById('remoteVideos').appendChild(el)
        }
    }

    componentWillReceiveProps(nextProps) {
        
    }
    
    log = (msg) => {
        this.setState((prevState) => {
            return {logs: [...prevState.logs, msg]}
        });
    }

    componentDidMount() {

        // var log = msg => {
        //     document.getElementById('logs').innerHTML += msg + '<br>'
        // }

        var that = this;
          
        navigator.mediaDevices.getUserMedia({ video: true, audio: false })
            .then((stream) => {
          
                document.getElementById('video1').srcObject = stream
          
                stream.getTracks().forEach(function(track) {
                    that.pc.addTrack(track, stream);
                });
          
                that.pc.createOffer().then((d) => { 
                    that.pc.setLocalDescription(d) 
                }).catch(that.log)
            }).catch(that.log) 
    }

    startSession = () => {
        let sd = document.getElementById('remoteSessionDescription').value
        if (sd === '') {
            return alert('Session Description must not be empty')
        }

        const buff = new Buffer(sd, 'base64');
        const remoteSession = buff.toString('utf8');
      
        try {
            this.pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(remoteSession)))
        } catch (e) {
            alert(e)
        }
    }


    render() {
        return (
            <div>
                Browser base64 Session Description<br />
                <textarea id="localSessionDescription" readOnly={true}></textarea> <br />

                Golang base64 Session Description<br />
                <textarea id="remoteSessionDescription"></textarea> <br/>
                <button onClick={this.startSession}> Start Session </button><br />

                <br />

                Video<br />
                <video id="video1" width="320" height="240" autoPlay muted></video> <br />


                Video CV<br />
                <div id="remoteVideos"></div> <br />

                Logs<br />
                <div id="logs">
                    {this.state.logs.map((log) => {
                        return <div className="log-message">{log}</div>
                    })
                    }
                </div>
            </div>
        )
    }
}

export default OpenCV;