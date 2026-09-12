import React, { Component } from "react";
import { Link, NavLink as RouterNavLink } from "react-router";
import {
  NavLink,
  Navbar,
  NavbarToggler,
  Collapse,
  Nav,
  NavItem,
  DropdownMenu,
  DropdownItem,
  DropdownToggle,
  UncontrolledDropdown,
} from "reactstrap";
import Config, { MenuItem } from "@engine/Config";
import Auth from "@controllers/auth";
import { t, getLocale, setLocale, locales, subscribe } from "@engine/i18n";

const iconStyle: React.CSSProperties = { height: "1.2em", verticalAlign: "middle", marginRight: 4 };

// A module/page's icon is one of:
//   a bare name ("home", "user-rights", …) — a cell of the /img/icons_bw.png
//     sprite sheet, picked out by the matching `.icon-<name>` class in
//     menu.css (that class sets only mask-position; size/masking is shared
//     via .menu-icon-sprite, applied alongside it below)
//   a path/URL ("/image/<uuid>", or similar) — an arbitrary image
//   "" (falsy) — no icon
const isImageIcon = (icon: string) => /^(?:https?:)?\//.test(icon);

// Render an item's label: "icon name", or just name, or just icon.
function label(item: MenuItem): React.ReactNode {
  if (!item.icon) return t(item.title);
  const icon = isImageIcon(item.icon) ? (
    <img src={item.icon} alt="" className="menu-icon" style={iconStyle} />
  ) : (
    <span className={`menu-icon-sprite icon-${item.icon}`} aria-hidden="true" />
  );
  if (item.title) return (<>{icon}{t(item.title)}</>);
  return icon;
}

interface MenuState {
  isOpen: boolean;
}

// The menu is server-driven: Config.getHead() are the top-level entries and
// Config.getSubmenus() the dropdown groups, both already privilege-filtered.
class Menu extends Component<{}, MenuState> {
  state: MenuState = { isOpen: false };
  private unsubscribeI18n?: () => void;

  // Class component, so useT()'s hook isn't available: subscribe manually and
  // force a re-render whenever the locale or a translation resolves.
  componentDidMount() {
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
  }

  toggle = () => this.setState((s) => ({ isOpen: !s.isOpen }));
  // Collapse the mobile menu after navigating, so picking a link doesn't leave
  // the open panel covering the page underneath it.
  close = () => this.setState({ isOpen: false });

  render() {
    const head = Config.getHead();
    const submenus = Config.getSubmenus();
    const authed = Auth.isAuthenticated();
    const avatar = Auth.getAvatar();
    // Profile gets its own slot on the right (showing the user's name instead
    // of "Profile"), so it's pulled out of the regular left-aligned items.
    const profileItem = head.find((i) => i.key === "profile");
    const leftHead = head.filter((i) => i.key !== "profile");

    const renderHeadItem = (item: MenuItem, idx: number): React.ReactNode => {
      // The login item becomes a Logout button for authenticated users.
      if (item.key === "login" && authed) {
        return (
          <NavItem key="logout">
            <NavLink
              href="#"
              onClick={(e: React.MouseEvent) => {
                e.preventDefault();
                this.close();
                Auth.logout();
              }}
            >
              <span className="menu-icon-sprite icon-logout" aria-hidden="true" />
              {t("Logout")}
            </NavLink>
          </NavItem>
        );
      }
      return (
        <NavItem key={"h" + idx}>
          <NavLink tag={RouterNavLink} to={item.path} onClick={this.close}>
            {label(item)}
          </NavLink>
        </NavItem>
      );
    };

    return (
      <div className="main-menu" style={menuDiv}>
        <Navbar color="dark" dark expand="md">
          {/* Brand mark — purely decorative (a plain Link, not NavLink, so it
              never picks up "active" nav styling); the "Home" nav item below
              still covers real/keyboard navigation. */}
          <Link to="/" className="menu-brand" onClick={this.close}>
            <img src="/img/favico_256.png" alt="" className="menu-brand-logo" />
          </Link>

          <NavbarToggler onClick={this.toggle} aria-label="Toggle navigation" />
          <Collapse isOpen={this.state.isOpen} navbar>
            <Nav navbar>
                {/* Home link — the Home page's href is "" (the root route "/"), so it
                  isn't in the data-driven head list; add it explicitly here. `end`
                  keeps it active only on exactly "/", not on every route. */}
              <NavItem>
                <NavLink tag={RouterNavLink} to="/" end onClick={this.close}>
                  <span className="menu-icon-sprite icon-home" aria-hidden="true" />
                  {t("Home")}
                </NavLink>
              </NavItem>

              {leftHead.map(renderHeadItem)}

              {Object.keys(submenus).map((title) => {
                const items = submenus[title];
                if (!items || items.length === 0) return null;
                return (
                  <UncontrolledDropdown key={"s" + title} setActiveFromChild>
                    <DropdownToggle tag="a" className="nav-link" caret>
                      {t(title)}
                    </DropdownToggle>
                    <DropdownMenu>
                      {items.map((item, key) => (
                        <DropdownItem key={key} tag={RouterNavLink} to={item.path} onClick={this.close}>
                          {label(item)}
                        </DropdownItem>
                      ))}
                    </DropdownMenu>
                  </UncontrolledDropdown>
                );
              })}
            </Nav>

            {/* Right-aligned account cluster: language, then avatar + name/role. */}
            <Nav className="ml-auto align-items-md-center" navbar>
              <NavItem>
                <select
                  className="form-select form-select-sm menu-locale"
                  value={getLocale()}
                  onChange={(e) => setLocale(e.target.value)}
                  aria-label={t("Language")}
                >
                  {locales.map((l) => (
                    <option key={l} value={l}>{l.toUpperCase()}</option>
                  ))}
                </select>
              </NavItem>

              {authed && profileItem && (
                <NavItem>
                  <NavLink tag={RouterNavLink} to={profileItem.path} onClick={this.close} className="menu-account">
                    {avatar ? (
                      <img src={avatar} alt="" className="menu-avatar" />
                    ) : (
                      <span className="menu-avatar menu-avatar-fallback">
                        <span className="menu-icon-sprite icon-profile" aria-hidden="true" />
                      </span>
                    )}
                    <span className="menu-account-text">
                      <span className="menu-account-name">{Auth.getUserName() || t("Profile")}</span>
                      <span className="menu-account-role">
                        {Auth.isAdmin() ? t("Administrator") : t("Member")}
                      </span>
                    </span>
                  </NavLink>
                </NavItem>
              )}
            </Nav>
          </Collapse>
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