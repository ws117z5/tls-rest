import React, {Component, useState} from 'react';
import { useNavigate, useNavigationParam } from 'react-router-dom'
import Functional from '../../controllers/functional';
import PageComponent from '../../controllers/PageComponent';
import Request from '../../controllers/request';
import "../container/Papers/papers.css"
import { Room } from '../container/Papers/Room';
import { Button, Form, FormGroup, Label, Input, FormText, Col } from 'reactstrap';

export default class PapersPage extends PageComponent {
    isPage = true;
    static href = 'papers'
    static title = 'Papers Game'

    static extraRoutes = [
        {
            href: '/papers/:uuid',
            component: Room
        }
    ]

    constructor(props, state) {
        super(props, state)

        let uuid = localStorage.getItem('pageUserId') ? localStorage.getItem('pageUserId') : (() => { 
            let uuid = Functional.guid(); 
            localStorage.setItem('pageUserId', uuid);
            return uuid;
        })();

        this.state = {
            loaded: false,
            roomId: "",
            //userId will be saved for all the sessions
            userId: uuid,
            rooms: [],
        }



    }

    updateRoomList = () => {
        Request.apiRequest('papers').then((res) => {
            //update rooms list
            res.data.Data.map((room) => {
                room.users = JSON.parse(room.users)
            })
            this.setState({
                loaded: true, 
                rooms: res.data.Data
            })
        })
    }

    componentWillMount() {
        this.updateRoomList();
        //this.setState({rooms: [Room]})
    }

    onRoomEnter = (id, userId) => {
        
    }

    onRoomExit = (rooms) => {
        this.setState({loaded: false}, () => {
            this.updateRoomList();
        })
        //this.setState({rooms: id})
    }

    componentDidMount() {
        
    }

    enterRoom = uuid => (e) => {

        //var userId = this.state.userId == "" ? Functional.guid() : currentUser.uuid;
        //this.setState({roomId: uuid, userId: userId})
        //props.onRoomEnter(uuid, this.state.userId);


        // Request.apiCall(`papers/${uuid}`, {...namePass, uuid: uuid}).then((res) => {
        //     //we can pass params to navigation destination component using location.state
        //      navigate(`/papers/` + uuid, {state: {user: userUuid}});
        // });

        this.props.navigate(`/papers/` + uuid, {state: {user: this.state.userId}});
    }

    render() {
        const { loaded , roomId, rooms, userId } = this.state;

        return (
            <div className="base">
            {!loaded ? <></> : 
                <div className="papers">
                    {this.state.currentUser != null &&
                        <div><span>Hello {this.state.name}</span></div>
                    }
                    <RoomAddNew {...this.props} userId={userId} onRoomEnter={this.onRoomEnter} onRoomExit={this.onRoomExit} /> 

                    {rooms.length > 0 && 
                        rooms.map((room, idx) => {
                            return (
                                <div className="room-list" key={idx}>
                                    <span>{room.name}</span>
                                    <span>{room.users.length}</span>
                                    <Button onClick={this.enterRoom(room.uuid)}>Enter</Button>
                                </div>
                            )
                        })
                    }
                </div>
            }
            </div>
        )
    }
}


export function RoomAddNew(props) {
    const [empty, setEmpty] = useState(true);
    const [namePass, setNamePass] = useState({name:"", password: ""});
    const [clicked, setClicked] = useState(false);

    const navigate = useNavigate();

    const setStateEmpty = name => (e) => {
        namePass[name] = e.target.value;

        setNamePass({...namePass});

        if (e.target.value != "") {
            setEmpty(false);
        } else {
            setEmpty(true);
        }
    }

    const onCreation = (e) => {
        //update parent
        //send to server 

        //create new room uuid
        var uuid = Functional.guid();

        props.onRoomEnter(uuid, props.userId);

        Request.apiCall(`papers/create`, {...namePass, uuid: uuid}).then((res) => {
            //we can pass params to navigation destination component using location.state
             navigate(`/papers/` + uuid, {state: {user: userUuid}});
        });
    }

    return (
        <div className="room-list">
            <span onClick={setClicked}>Create New</span>
                    
                    {clicked && 
                        <>
                        <Input type="text" name="roomName" onChange={setStateEmpty('name')} placeholder="Room name" />
                        <Input type="password" name="roomPass" onChange={setStateEmpty('password')} placeholder="Room password" />
                        <Button onClick={onCreation} disabled={empty}>Create Room</Button>
                        </>
                    }
        </div>
    );
}