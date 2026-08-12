import React, { Component } from "react";
import { NavLink as RouterNavLink } from "react-router";
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
    // Backend modules are already access-filtered by the server; custom pages
    // were filtered by Config at load time. So the menu just renders what's there.
    const modules = Config.getModules();
    const navPages = Config.getNavPages();
    const pages = Config.getPages();

    return (
      <div className="main-menu" style={menuDiv}>
        <Navbar color="dark" dark expand="md">
          <Nav className="ml-auto" navbar>
            {modules.map((module, idx) => (
              <NavItem key={"m" + idx}>
                <NavLink tag={RouterNavLink} to={"/" + module.href}>
                  {module.title}
                </NavLink>
              </NavItem>
            ))}

            {navPages.map((page, idx) => {
              const visible =
                typeof page.condition === "undefined" ||
                page.condition(this.state);
              if (!visible) return null;
              return (
                <NavItem key={"p" + idx}>
                  <NavLink tag={RouterNavLink} to={"/" + page.href}>
                    {page.title}
                  </NavLink>
                </NavItem>
              );
            })}

            {pages.length > 0 && (
              <UncontrolledDropdown setActiveFromChild>
                <DropdownToggle tag="a" className="nav-link" caret>
                  Pages
                </DropdownToggle>
                <DropdownMenu>
                  {pages.map((page, key) => (
                    <DropdownItem key={key} tag={RouterNavLink} to={"/pages/" + page.href}>
                      {page.title}
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