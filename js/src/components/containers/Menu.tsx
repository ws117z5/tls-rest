import React, { Component } from "react";
import {
  NavLink,
  Navbar,
  Nav,
  NavItem,
  DropdownMenu,
  DropdownItem,
  DropdownToggle,
  UncontrolledDropdown,
} from "reactstrap";
import "./menu.css";
import Config from "@engine/Config";
import Auth from "@controllers/auth";

interface MenuState {
  someVariable: boolean;
  money: boolean;
  auth: boolean;
}

class Menu extends Component<{}, MenuState> {
  constructor(props: {}) {
    super(props);
    this.state = {
      someVariable: true,
      money: true,
      auth: false,
    };
  }

  render() {
    const Pages = Config.getPages();

    return (
      <div className="main-menu" style={menuDiv}>
        <Navbar color="light" light expand="md">
          <Nav className="ml-auto" navbar>
            {Config.getAll()?.map((module: any, idx: number) => {
              // Hide modules the current user has no rights to (auth/admin flags
              // plus per-module rights from the backend rights/groups system).
              if (!Auth.canAccessModule(module)) {
                return null;
              }
              if (
                !module.isPage &&
                ((module.condition && module.condition(this.state)) ||
                  typeof module.condition === "undefined")
              ) {
                return (
                  <NavItem key={idx}>
                    <NavLink href={"/" + module.href} activeclassname="active">
                      {module.title}
                    </NavLink>
                  </NavItem>
                );
              }
              return null;
            })}
            { Pages && Pages.filter((m: any) => Auth.canAccessModule(m)).length > 0 && (
              <UncontrolledDropdown setActiveFromChild>
                <DropdownToggle tag="a" className="nav-link" caret>
                  Pages
                </DropdownToggle>
                <DropdownMenu>
                  {Pages
                    .filter((m: any) => Auth.canAccessModule(m))
                    .map((module: any, key: number) => (
                    <DropdownItem
                      key={key}
                      tag="a"
                      href={"/pages/" + module.href}
                    >
                      {module.title}
                    </DropdownItem>
                  ))}
                </DropdownMenu>
              </UncontrolledDropdown>
            )}
          </Nav>
        </Navbar>
      </div>
    );
  }
}

const menuDiv: React.CSSProperties = {
  zIndex: 100,
  position: "sticky",
  top: 0,
  left: 0,
  width: "100%",
};

export default Menu;