import React, {Component} from "react"

import Funcional from "../../../controllers/functional"
import { Button, Form, FormGroup, Label, Input, FormText, Col } from 'reactstrap';
import Log from "../../../controllers/log"
import Request from "../../../controllers/request";


export default class Participant extends Component {
    static iceServers = [
        {urls: "stun:stun.l.google.com:19382?transport=udp"}
    ];

    static defaultProps = {
        uuid: "",
        name: "",
        word: "",
        pc: null,
        dc: null,
        localSession: {}
    }

    constructor(props, state) {
        super(props, state)

        this.state = {
            userValid: false,
            uuid: props.user.uuid,
            name: props.user.name,
            word: props.user.word,
            pc: new RTCPeerConnection(Participant.iceServers),
        }

        this.state.userInited = (props.user.name != "" && props.user.word != "")

    }

    initUser = (e) => {

        //todo init video then set state
        const user = {
            //uuid: Funcional.guid(),
            name: this.state.name,
            word: this.state.word,
            uuid: this.state.uuid,
            userInited: true
        }

        //localStorage.setItem('papresState', JSON.stringify())

        var _state = this.state;


        /*
        Here we init rules for the client video connections.
        */
        _state.pc.oniceconnectionstatechange = e => {
            Log.log(_state.pc.iceConnectionState)
            Log.log(e)
        }

        _state.pc.onicecandidate = event => {
            if (event.candidate === null) {

                _state.localSession = _state.pc.localDescription
                Log.log(_state.pc.localDescription)
                //update parent
                this.props.onUserInit(user.name, user.word, user.uuid, _state.pc.localDescription)
            }
        }

        navigator.mediaDevices.getUserMedia({ video: true, audio: false }).then(stream => {
            stream.getTracks().forEach(track => {
                _state.pc.addTrack(track, stream);
            });

            //attach local video 
            let el = document.getElementById(_state.uuid)
            el.srcObject = stream
            el.autoplay = true

            _state.pc.createOffer()
                .then(d => _state.pc.setLocalDescription(d))
                .catch(Log.log)

        }).catch(Log.log)

        
        this.props.onUserInit(user.name, user.word, user.uuid, "")
        this.setState(user);
    }

    getLocalSession = () => {
        return this.state.localSession;
    }
    
    onUserChange = name => (e) => {
        this.state[name] = e.target.value;

        var newState = {};
        newState[name] = e.target.value
        
        if(this.state.name != "" && this.state.word != "") {
            newState.userValid = true;
        } 

        if (this.state.name == "" || this.state.word == "") {
            newState.userValid = false;
        }

        this.setState(newState);
        
    }

    componentDidMount() {
        if (this.state.userInited) {
            this.initUser();
        }
    }

    render() {
        return (
            <div className="participant-list">
            {!this.state.userInited ? 
                <>
                    <Input type="text" value={this.state.name} name="userName" onChange={this.onUserChange('name')} placeholder="User name" />
                    <video id={this.state.uuid}></video>
                    <Input type="text" value={this.state.word} name="userWord" onChange={this.onUserChange('word')} placeholder="Word pick" />
                    <Button onClick={this.initUser} disabled={!this.state.userValid}>Save</Button>
                </> 
                :
                <>
                    { /* video goes here */ }
                    <span>{this.state.name}</span>
                    <video id={this.state.uuid}></video>
                    <span>{this.state.word}</span>
                    <span>{this.state.localSession}</span>
                </>
            }

            </div>
        )
    }
}

