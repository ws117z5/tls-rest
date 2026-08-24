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
import "@css/menu.css";
import Config, { MenuItem } from "@engine/Config";
import Auth from "@controllers/auth";

const iconStyle: React.CSSProperties = { height: "1.2em", verticalAlign: "middle", marginRight: 4 };
const avatarStyle: React.CSSProperties = { height: 32, width: 32, borderRadius: "50%", objectFit: "cover" };

// Render an item's label: "icon name", or just name, or just icon.
function label(item: MenuItem): React.ReactNode {
  const icon = item.icon ? <img src={item.icon} alt="" className="menu-icon" style={iconStyle} /> : null;
  if (icon && item.title) return (<>{icon}{item.title}</>);
  if (icon) return icon;
  return item.title;
}

// The menu is server-driven: Config.getHead() are the top-level entries and
// Config.getSubmenus() the dropdown groups, both already privilege-filtered.
class Menu extends Component<{}, {}> {
  render() {
    const head = Config.getHead();
    const submenus = Config.getSubmenus();
    const authed = Auth.isAuthenticated();
    const avatar = Auth.getAvatar();

    const renderHeadItem = (item: MenuItem, idx: number): React.ReactNode => {
      // The login item becomes a Logout button for authenticated users.
      if (item.key === "login" && authed) {
        return (
          <NavItem key="logout">
            <NavLink
              href="#"
              onClick={(e: React.MouseEvent) => {
                e.preventDefault();
                Auth.logout();
              }}
            >
              Logout
            </NavLink>
          </NavItem>
        );
      }
      return (
        <NavItem key={"h" + idx}>
          <NavLink tag={RouterNavLink} to={item.path}>
            {label(item)}
          </NavLink>
        </NavItem>
      );
    };

    return (
      <div className="main-menu" style={menuDiv}>
        <Navbar color="dark" dark expand="md">
          <Nav className="ml-auto" navbar>
            {head.map(renderHeadItem)}

            {Object.keys(submenus).map((title) => {
              const items = submenus[title];
              if (!items || items.length === 0) return null;
              return (
                <UncontrolledDropdown key={"s" + title} setActiveFromChild>
                  <DropdownToggle tag="a" className="nav-link" caret>
                    {title}
                  </DropdownToggle>
                  <DropdownMenu>
                    {items.map((item, key) => (
                      <DropdownItem key={key} tag={RouterNavLink} to={item.path}>
                        {label(item)}
                      </DropdownItem>
                    ))}
                  </DropdownMenu>
                </UncontrolledDropdown>
              );
            })}

            {authed && avatar && (
              <NavItem>
                <img src={avatar} alt="avatar" className="menu-avatar" style={avatarStyle} />
              </NavItem>
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