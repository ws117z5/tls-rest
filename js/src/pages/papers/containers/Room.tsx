import React, { ChangeEvent } from 'react';
import PageComponent from "@engine/containers/PageComponent";
import { Button } from 'reactstrap';
import Participant from './Participant';
import Log from "@engine/controllers/log";
import Request from '@engine/controllers/request';

// 1. Declare custom global properties on Window if needed
declare global {
    interface Window {
        startSession?: () => void;
    }
}

interface CurrentUser {
    name: string;
    word: string;
    uuid: string;
    dc?: RTCDataChannel | null;
    pc: RTCPeerConnection;
}

interface RoomState {  
    loaded: boolean;
    name: string;
    uuid: string;
    users: any[];
    currentUser: CurrentUser;
    size: number;
    clicked?: boolean; 
    notEmpty?: boolean;
}

interface RoomProps {
    params: {
        uuid?: string;
        [key: string]: any;
    };
    location: {
        state?: {
            user?: string;
            [key: string]: any;
        };
        [key: string]: any;
    };
    navigate: (to: string, options?: any) => void;
}   

export class Room extends PageComponent<RoomProps, RoomState> {
    notEmpty = true;

    constructor(props: RoomProps) {
        super(props);

        // Extract router and state variables safely
        const roomUUID = props.params?.uuid || "";
        const userUUID = props.location?.state?.user || "";

        let currentUser: CurrentUser = {
            name: "",
            word: "",
            uuid: userUUID,
            pc: new RTCPeerConnection(Participant.configuration),
            dc: null
        };

        // In case of page refresh, restore stored user details
        const stored = localStorage.getItem(`${roomUUID}`);
        if (stored) {
            try {
                const parsedStored = JSON.parse(stored);
                if (currentUser.uuid === "" || parsedStored.uuid === userUUID) {
                    currentUser = {
                        ...currentUser,
                        ...parsedStored
                    };
                }
            } catch (e) {
                Log.log("Failed to parse stored room state:", e);
            }
        }

        this.state = {
            loaded: false,
            name: "",
            uuid: roomUUID,
            users: [],
            currentUser: currentUser,
            size: 0,
            notEmpty: true
        };
    }

    temp = () => {
        const log = (msg: any) => {
            console.log(msg);
        };

        for (const remoteUser of this.state.users) {
            if (remoteUser.uuid === this.state.currentUser.uuid) {
                continue;
            }

            const remotePC = new RTCPeerConnection({
                iceServers: [
                    {
                        urls: 'stun:stun.l.google.com:19302'
                    }
                ]
            });

            remotePC.addTransceiver('video');
            remotePC.createOffer()
                .then(d => remotePC.setLocalDescription(d))
                .catch(log);

            remotePC.ontrack = (event: RTCTrackEvent) => {
                const el = document.getElementById(remoteUser.uuid) as HTMLVideoElement | null;
                if (el) {
                    el.srcObject = event.streams[0];
                    el.autoplay = true;
                    el.controls = false;
                }
            };
        }

        window.startSession = () => {
            const sdInput = document.getElementById('remoteSessionDescription') as HTMLInputElement | null;
            const sd = sdInput?.value || '';
            
            if (sd === '') {
                return alert('Session Description must not be empty');
            }

            try {
                // Handled via session description exchange
            } catch (e) {
                alert(e);
            }
        };
    };

    async componentDidMount() {
        // A peer connection created in the constructor can be closed before this
        // async mount finishes (e.g. React StrictMode's mount/unmount/remount in
        // dev, which closes it in componentWillUnmount). Use a fresh connection
        // if the existing one is gone, so createDataChannel never runs on a
        // 'closed' connection.
        let pc = this.state.currentUser.pc;
        if (!pc || pc.signalingState === "closed") {
            pc = new RTCPeerConnection(Participant.configuration);
            this.setState((prev) => ({ currentUser: { ...prev.currentUser, pc } }));
        }

        pc.oniceconnectionstatechange = (e: Event) => {
            Log.log(pc.iceConnectionState);
            Log.log(e);
        };

        pc.onicecandidate = (event: RTCPeerConnectionIceEvent) => {
            if (event.candidate === null && pc.localDescription) {
                const postParams = {
                    client: JSON.stringify(pc.localDescription)
                };

                Request.apiCall(`papers/${this.state.uuid}/${this.state.currentUser.uuid}`, postParams)
                    .then((res: any) => {
                        pc.setRemoteDescription(new RTCSessionDescription(res.data));
                    })
                    .catch(Log.log);
            }
        };

        pc.onnegotiationneeded = () => {
            pc.createOffer()
                .then(d => pc.setLocalDescription(d))
                .catch(Log.log);
        };

        // Initialize DataChannel on the Peer Connection (skip if it closed while
        // this async mount was in flight).
        if (pc.signalingState === "closed") {
            return;
        }
        const sendChannel = pc.createDataChannel(this.state.uuid);
        sendChannel.onclose = () => Log.log('sendChannel has closed');
        sendChannel.onopen = () => Log.log('sendChannel has opened');
        sendChannel.onmessage = (e: MessageEvent) => {
            Log.log(`Message from DataChannel '${sendChannel.label}' payload '${e.data}'`);
        };

        // Update state with initialized data channel reference
        this.setState(prevState => ({
            currentUser: {
                ...prevState.currentUser,
                dc: sendChannel
            }
        }));
    }

    createNew = () => {
        this.setState({ clicked: true });
    };

    onRoomChange = (e: ChangeEvent<HTMLInputElement>) => {
        this.setState({ notEmpty: e.target.value !== "" });
    };

    onUserChange = (fieldName: keyof CurrentUser) => (e: ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value;
        this.setState(prevState => ({
            currentUser: {
                ...prevState.currentUser,
                [fieldName]: value
            }
        }));
    };

    onUserComponentInit = (uuid: string) => {
        this.setState(prevState => ({
            currentUser: {
                ...prevState.currentUser,
                uuid: uuid
            }
        }));
    };

    setCurrentUser = () => {
        // Keeps state synchronized if modified via external handlers
        this.setState(prevState => ({
            currentUser: prevState.currentUser
        }));
    };

    exit = () => {
        localStorage.removeItem(`${this.state.uuid}`);
        this.props.navigate('/pages/papers');
    };

    componentWillUnmount() {
        // Close the room-level connection; each Participant releases its own
        // camera + peer connection in its own componentWillUnmount.
        const { pc, dc } = this.state.currentUser;
        try {
            dc?.close();
        } catch (e) {
            Log.log(e);
        }
        try {
            pc?.close();
        } catch (e) {
            Log.log(e);
        }
    }

    onUserInit = (name: string, word: string, uuid: string, pcld?: RTCSessionDescriptionInit | "") => {
        const currentUserData = {
            name,
            word,
            uuid,
            pcld
        };

        localStorage.setItem(`${this.state.uuid}`, JSON.stringify(currentUserData));
    };

    render() {
        const { currentUser, users } = this.state;

        return (
            <div className="base">
                <div className="page-actions">
                    <Button onClick={this.exit}>Exit</Button>
                </div>
                <div className="room-view">
                    <Participant user={currentUser} onUserInit={this.onUserInit} />
                    {users.length > 0 &&
                        users.map((user) => {
                            if (user.uuid !== currentUser.uuid) {
                                return (
                                    <Participant 
                                        key={user.uuid || user.name} 
                                        user={user} 
                                        onUserInit={this.onUserInit} 
                                    />
                                );
                            }
                            return null;
                        })
                    }
                </div>
            </div>
        );
    }
}

export default Room;