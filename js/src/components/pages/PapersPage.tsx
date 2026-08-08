import React, {Component, useState, ChangeEvent, MouseEvent } from 'react';
import { useNavigate, NavigateFunction, Params, Location } from 'react-router'
import Functional from '@controllers/functional';
import PageComponent, { PageComponentProps, PageComponentState } from '@engine/PageComponent';
import Request from '@controllers/request';
import "@containers/Papers/papers.css"
import { Room } from '@containers/Papers/Room';
import { Button, Form, FormGroup, Label, Input, FormText, Col } from 'reactstrap';

interface PapersPageState extends PageComponentState {
    rooms: any[];
    roomId: string;
    userId: string;
    name: string;
    currentUser: any;
    userUuid: string;
}

export default class PapersPage extends PageComponent<PageComponentProps, PapersPageState> {
    protected isPage = true;
    protected href = 'papers';
    protected title = 'Papers Game';

    static extraRoutes = [
        {
            href: '/papers/:uuid',
            component: Room
        }
    ]

    constructor(props, state) {
        super(props)

        let uuid = localStorage.getItem('pageUserId') ? localStorage.getItem('pageUserId') : (() => { 
            let uuid = Functional.guid(); 
            localStorage.setItem('pageUserId', uuid);
            return uuid;
        })();

        this.state = {
            ...this.state,
            rooms: [],
            roomId: "",
            userId: uuid || "",
            userUuid: uuid || "",
            name: "",
            currentUser: null,
        };
    }

    updateRoomList = () => {
        Request.apiRequest('papers').then((res) => {
            //update rooms list
            res.data.Data.map((room) => {
                room.users = JSON.parse(room.users)
            })
            this.setState({
                loading: false, 
                rooms: res.data.Data
            })
        })
    }

    async componentDidMount() {
        // Papers-specific load (parses room.users JSON). We intentionally do not
        // call super.componentDidMount(), which would issue a second GET /papers.
        this.updateRoomList();
        //this.setState({rooms: [Room]})
    }

    onRoomEnter = (id, userId) => {
        
    }

    onRoomExit = (rooms) => {
        this.setState({loading: false}, () => {
            this.updateRoomList();
        })
        //this.setState({rooms: id})
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
        const { loading , roomId, rooms, userId } = this.state;

        return (
            <div className="base">
            {loading ? <></> : 
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


export interface RoomAddNewProps {
  userId: string;
  onRoomEnter: (uuid: string, userId: string) => void;
  onRoomExit?: (rooms?: any) => void; // Added to fix Property 'onRoomExit' does not exist error
  navigate?: NavigateFunction;
  location?: Location;
  params?: Params;
}

export interface RoomAddNewState {
  empty: boolean;
  namePass: {
    name: string;
    password: string;
  };
  clicked: boolean;
}

export class RoomAddNewClass extends Component<RoomAddNewProps, RoomAddNewState> {
  constructor(props: RoomAddNewProps) {
    super(props);

    this.state = {
      empty: true,
      namePass: {
        name: "",
        password: ""
      },
      clicked: false
    };
  }

  setStateEmpty =
    (field: "name" | "password") =>
    (e: ChangeEvent<HTMLInputElement>): void => {
      const val = e.target.value;

      this.setState((prevState) => {
        const nextNamePass = {
          ...prevState.namePass,
          [field]: val
        };

        const isBothEmpty =
          nextNamePass.name.trim() === "" &&
          nextNamePass.password.trim() === "";

        return {
          namePass: nextNamePass,
          empty: isBothEmpty
        };
      });
    };

  onCreation = (): void => {
    const { onRoomEnter, userId, navigate } = this.props;
    const { namePass } = this.state;

    // Create new room uuid
    const uuid = Functional.guid();

    onRoomEnter(uuid, userId);

    Request.apiCall(`papers/create`, { ...namePass, uuid: uuid })
      .then((_res: unknown) => {
        if (navigate) {
          navigate(`/papers/${uuid}`, { state: { user: userId } });
        }
      })
      .catch((err: unknown) => {
        // The server returns an error (e.g. 400/500 from CreateRoom) when the
        // room can't be persisted. Handle it instead of leaving an unhandled
        // promise rejection, and don't navigate to a room that wasn't created.
        console.error("Failed to create room:", err);
      });
  };

  onSubmit = (e: React.FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    this.onCreation();
  };

  toggleClicked = (): void => {
    this.setState((prevState) => ({
      clicked: !prevState.clicked
    }));
  };

  render() {
    const { clicked, empty } = this.state;

    return (
      <div className="room-list">
        <span onClick={this.toggleClicked}>Create New</span>

        {clicked && (
          <Form onSubmit={this.onSubmit}>
            <Input
              type="text"
              name="roomName"
              autoComplete="off"
              onChange={this.setStateEmpty("name")}
              placeholder="Room name"
            />
            <Input
              type="password"
              name="roomPass"
              autoComplete="new-password"
              onChange={this.setStateEmpty("password")}
              placeholder="Room password"
            />
            <Button type="submit" disabled={empty}>
              Create Room
            </Button>
          </Form>
        )}
      </div>
    );
  }
}

// Wrapper HOC to pass `navigate` hook into class component
export function RoomAddNew(props: Omit<RoomAddNewProps, "navigate">) {
  const navigate = useNavigate();
  return <RoomAddNewClass {...props} navigate={navigate} />;
}