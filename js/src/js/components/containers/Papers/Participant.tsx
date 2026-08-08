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
            { urls: "stun:stun.l.google.com:19302" }
        ]
    };

    // Held so the camera can be released when the participant leaves the room.
    private localStream: MediaStream | null = null;
    // Ref to the local preview <video> — reliable across re-renders, unlike getElementById.
    private videoRef = React.createRef<HTMLVideoElement>();
    private initialized = false;

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
        // Avoid re-running getUserMedia (e.g. Save + componentDidMount, or a
        // dev-mode double mount) — a second capture of the same camera can hand
        // back a black stream.
        if (this.initialized) {
            return;
        }
        this.initialized = true;

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
                this.localStream = stream;
                stream.getTracks().forEach(track => {
                    pc.addTrack(track, stream);
                });

                // Attach to the actual rendered <video> via its ref.
                this.attachStream();

                return pc.createOffer();
            })
            .then(d => pc.setLocalDescription(d))
            .catch(Log.log);

        this.props.onUserInit(name, word, uuid, "");
        this.setState({ userInited: true });
    };

    // attachStream binds the captured camera stream to the current preview
    // <video>. It is idempotent and re-run on every update, so the stream is
    // (re)attached whenever the element mounts or React swaps it out. A local
    // self-preview must be muted + playsInline so the browser autoplays it
    // without a user gesture.
    attachStream = () => {
        const el = this.videoRef.current;
        if (!el || !this.localStream || el.srcObject === this.localStream) {
            return;
        }

        el.srcObject = this.localStream;
        el.muted = true;
        el.autoplay = true;
        el.playsInline = true;

        const tryPlay = () => {
            el.play()
                .then(() => Log.log(`local video playing ${el.videoWidth}x${el.videoHeight}`))
                .catch((err) => Log.log("local video play() rejected:", err));
        };
        // Play once metadata is available (more reliable than an immediate call),
        // and also try immediately in case metadata is already there.
        el.onloadedmetadata = tryPlay;
        tryPlay();

        // Diagnostics: is the camera actually producing frames? A blank/black
        // preview with enabled=true, muted=false, state="live" but 0x0 dimensions
        // points at the camera/source, not the wiring.
        this.localStream.getVideoTracks().forEach((t) => {
            Log.log(`local video track: enabled=${t.enabled} muted=${t.muted} state=${t.readyState} label=${t.label}`);
        });
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

    componentDidUpdate() {
        // Re-bind the stream if the preview <video> was (re)mounted, e.g. when the
        // form is replaced by the joined view.
        this.attachStream();
    }

    componentWillUnmount() {
        // Release the camera and tear down the connection when leaving the room.
        if (this.localStream) {
            this.localStream.getTracks().forEach(track => track.stop());
            this.localStream = null;
        }
        const el = this.videoRef.current;
        if (el) {
            el.srcObject = null;
        }
        try {
            this.state.pc.close();
        } catch (e) {
            Log.log(e);
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
                        <video ref={this.videoRef} id={this.state.uuid} autoPlay muted playsInline style={{ width: "100%", minHeight: 180, background: "#000" }}></video>
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
                        <video ref={this.videoRef} id={this.state.uuid} autoPlay muted playsInline style={{ width: "100%", minHeight: 180, background: "#000" }}></video>
                        <span>{this.state.word}</span>
                    </>
                )}
            </div>
        );
    }
}