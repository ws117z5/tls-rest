import React, { Component, ChangeEvent, MouseEvent } from "react";
import Funcional from "@controllers/functional";
import { Button, Input } from 'reactstrap';
import Log from "@controllers/log";
import Request from "@controllers/request";

export interface ParticipantState {
    userValid: boolean;
    userInited: boolean;
    uuid: string;
    name: string;
    word: string;
    pc: RTCPeerConnection;
    dc?: RTCDataChannel;
    localSession?: RTCSessionDescriptionInit;
}

export interface ParticipantProps {
    user: {
        uuid: string;
        name: string;
        word: string;
    };
    onUserInit: (name: string, word: string, uuid: string, localSession?: RTCSessionDescriptionInit | "") => void;
}

export default class Participant extends Component<ParticipantProps, ParticipantState> {
    static configuration: RTCConfiguration = {
        iceServers: [
            { urls: "stun:stun.l.google.com:19302?transport=udp" }
        ]
    };

    constructor(props: ParticipantProps) {
        super(props);

        const pc = new RTCPeerConnection(Participant.configuration);

        this.state = {
            userValid: false,
            uuid: props.user.uuid,
            name: props.user.name,
            word: props.user.word,
            pc: pc,
            userInited: (props.user.name !== "" && props.user.word !== "")
        };
    }

    initUser = (e?: MouseEvent<HTMLButtonElement>) => {
        const { pc, name, word, uuid } = this.state;

        /*
        Configure WebRTC peer connection callbacks
        */
        pc.oniceconnectionstatechange = (event: Event) => {
            Log.log(pc.iceConnectionState);
            Log.log(event);
        };

        pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
            if (event.candidate === null && pc.localDescription) {
                const localDesc = pc.localDescription;

                // Safely update state using setState instead of mutating this.state
                this.setState({ localSession: localDesc });
                Log.log(localDesc);

                // Update parent component
                this.props.onUserInit(name, word, uuid, localDesc);
            }
        };

        navigator.mediaDevices.getUserMedia({ video: true, audio: false })
            .then(stream => {
                stream.getTracks().forEach(track => {
                    pc.addTrack(track, stream);
                });

                // Attach local video stream safely with HTMLVideoElement type assertion
                const el = document.getElementById(uuid) as HTMLVideoElement | null;
                if (el) {
                    el.srcObject = stream;
                    el.autoplay = true;
                }

                return pc.createOffer();
            })
            .then(d => pc.setLocalDescription(d))
            .catch(Log.log);

        this.props.onUserInit(name, word, uuid, "");
        this.setState({ userInited: true });
    };

    getLocalSession = (): RTCSessionDescriptionInit | undefined => {
        return this.state.localSession;
    };
    
    onUserChange = (fieldName: 'name' | 'word') => (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value;

        this.setState((prevState) => {
            const nextName = fieldName === 'name' ? value : prevState.name;
            const nextWord = fieldName === 'word' ? value : prevState.word;
            const isValid = nextName !== "" && nextWord !== "";

            return {
                [fieldName]: value,
                userValid: isValid
            } as Pick<ParticipantState, 'name' | 'word' | 'userValid'>;
        });
    };

    componentDidMount() {
        if (this.state.userInited) {
            this.initUser();
        }
    }

    render() {
        return (
            <div className="participant-list">
                {!this.state.userInited ? (
                    <>
                        <Input 
                            type="text" 
                            value={this.state.name} 
                            name="userName" 
                            onChange={this.onUserChange('name')} 
                            placeholder="User name" 
                        />
                        <video id={this.state.uuid}></video>
                        <Input 
                            type="text" 
                            value={this.state.word} 
                            name="userWord" 
                            onChange={this.onUserChange('word')} 
                            placeholder="Word pick" 
                        />
                        <Button onClick={this.initUser} disabled={!this.state.userValid}>Save</Button>
                    </> 
                ) : (
                    <>
                        <span>{this.state.name}</span>
                        <video id={this.state.uuid}></video>
                        <span>{this.state.word}</span>
                        <span>{this.state.localSession?.sdp}</span>
                    </>
                )}
            </div>
        );
    }
}