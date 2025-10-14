import React, { useState } from 'react'
import { useNavigate, useNavigationParam } from 'react-router-dom'

import PageComponent from "../../../controllers/PageComponent";
import { Button, Form, FormGroup, Label, Input, FormText, Col } from 'reactstrap';
import Functional from '../../../controllers/functional';
import Request from '../../../controllers/request';
import Participant from './Participant'
import Log from "../../../controllers/log"

export class Room extends PageComponent {
    notEmpty = true;

    /**
     * 
     * @param {
     *      params: any, 
     *      location: any
     * } props 
     * @param {*} state 
     */
    constructor(props, state) {
        super(props, state)

        //passed from router and location state
        const roomUUID = props.params.uuid;
        const userUUID = props.location.state.user;

        this.state = {
            //mode list
            loaded: false,
            name: "",
            uuid: roomUUID,
            users: [],

            currentUser: {
                name: "",
                word: "",
                uuid: userUUID,
                dc: new RTCPeerConnection(Participant.iceServers), 
                pc: new RTCPeerConnection(Participant.iceServers), 
            },
            size: 0
        }

        //in case of page refresh
        let _stored = localStorage.getItem(`${roomUUID}`);
        if (_stored) {
            let __stored = JSON.parse(_stored);
            if (this.state.currentUser.uuid == "" || __stored.uuid == userUUID) {
                this.state.currentUser = __stored;
            }
        }
        
    }

    temp = () => {

        var log = (msg) => {
            console.log(msg);
        }

        for (const remoteUser of this.state.users) {
            // init remote user video
            if (remoteUser.uuid == this.state.currentUser.uuid) {
                continue;
            }

            let remotePC = new RTCPeerConnection({
                iceServers: [
                  {
                    urls: 'stun:stun.l.google.com:19302'
                  }
                ]
            })

            
            remotePC.addTransceiver('video')
            remotePC.createOffer()
                .then(d => pc.setLocalDescription(d))
                .catch(log)
        
            remotePC.ontrack = function (event) {
                var el = document.getElementById(remoteUser.uuid)
                el.srcObject = event.streams[0]
                el.autoplay = true
                el.controls = false
            }


            remotePC.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
        }
        
        
        window.startSession = () => {
            let sd = document.getElementById('remoteSessionDescription').value
            if (sd === '') {
                return alert('Session Description must not be empty')
            }
        
            try {
                remotePC.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
            } catch (e) {
                alert(e)
            }
        }
    
        // let btns = document.getElementsByClassName('createSessionButton')
        // for (let i = 0; i < btns.length; i++) {
        //     btns[i].style = 'display: none'
        // }
    
        //document.getElementById('signalingContainer').style = 'display: block'
    }


    componentDidMount() {
        /*
        Here we init rules for the client-server connection for future negotiation.
        */
       const {name, word, uuid, dc} = this.state.currentUser;

       dc.oniceconnectionstatechange = e => {
            Log.log(_state.pc.iceConnectionState)
            Log.log(e)
        }

        dc.onicecandidate = event => {
            if (event.candidate === null) {

                //_state.localSession = _state.pc.localDescription
                let postParams = {
                    client: JSON.stringify(dc.localDescription)
                }
                Request.apiCall(`papers/${this.state.uuid}/${this.state.currentUser.uuid}`, postParams).then((res) => {
                    dc.setRemoteDescription(new RTCSessionDescription(res.data))
                }).catch(Log.log)
                //Log.log(_state.pc.localDescription)
                //update parent
                //this.props.onUserInit(user.name, user.word, user.uuid, _state.pc.localDescription)
            }
        }

        dc.onnegotiationneeded = e => {
            dc.createOffer().then(d => dc.setLocalDescription(d)).catch(Log.log)
        }


        let sendChannel = dc.createDataChannel(this.state.uuid)
        sendChannel.onclose = () => console.log('sendChannel has closed')
        sendChannel.onopen = () => console.log('sendChannel has opened')
        sendChannel.onmessage = e => {
            Log.log(`Message from DataChannel '${sendChannel.label}' payload '${e.data}'`)
        }
    }
    
    createNew = (e) => {
        this.setState({clicked: true});
    }


    onRoomChange = (e) => {
        if (e.target.value != "") {
            this.setState({notEmpty: true});
        } else {
            this.setState({notEmpty: false});
        }
    }

    onUserChange = name => (e) => {
        this.currentUser[name] = e.target.value;
    }

    onUserComponentInit = (uuid) => {
        this.currentUser.uuid = uuid;
        this.setState({currentUser: this.currentUser})
    }

    /**
     * 
     * @param {*} name 
     * @returns 
     */
    setCurrentUser = name => () => {
        this.setState({currentUser: this.currentUser})
    }

    exit = () => {
        localStorage.removeItem(`${this.state.uuid}`)
        this.props.navigate('/papers');
        //this.setState({mode: "list"})
    }

    /**
     * 
     * @param {string} name 
     * @param {string} word 
     * @param {string} uuid 
     * @param {RTCSessionDescription} pcld
     */
    onUserInit = (name, word, uuid, pcld) => {
        let currentUser = {
            name: name, 
            word: word, 
            uuid: uuid,
            pcld: pcld 
        }

        localStorage.setItem(`${this.state.uuid}`, JSON.stringify(currentUser));

        //this.participants.
    }

    

    render() {
        return(
            <div className="base">
                <div className='page-actions'>
                    <Button onClick={this.exit}>Exit</Button>
                </div>
                <div className="room-view">
                    <Participant user={this.state.currentUser} onUserInit={this.onUserInit}></Participant>
                    {this.state.users.length > 0 &&
                        this.state.users.map((user) => {
                            if(user.uuid != this.currentUser.uuid) {
                                return <Participant user={user} onUserInit={this.onUserInit} />;
                            }
                        })
                    }
                </div>
            </div>
        )
    }
}
