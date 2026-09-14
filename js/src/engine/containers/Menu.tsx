import React, { Component, useEffect, useRef, useState } from "react";
import { Link, NavLink as RouterNavLink } from "react-router";
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

// Replaces reactstrap's UncontrolledDropdown/DropdownToggle/DropdownMenu —
// same click-to-toggle, click-outside-to-close behavior, plain Bootstrap
// dropdown markup (styled by base.css).
interface NavDropdownProps {
  title: string;
  items: MenuItem[];
  onNavigate: () => void;
}
const NavDropdown: React.FC<NavDropdownProps> = ({ title, items, onNavigate }) => {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLLIElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("click", onDocClick);
    return () => document.removeEventListener("click", onDocClick);
  }, [open]);

  return (
    <li className="nav-item dropdown" ref={ref}>
      <a
        href="#"
        className="nav-link dropdown-toggle"
        onClick={(e) => {
          e.preventDefault();
          setOpen((o) => !o);
        }}
      >
        {title}
      </a>
      <div className={`dropdown-menu${open ? " show" : ""}`}>
        {items.map((item, key) => (
          <RouterNavLink
            key={key}
            to={item.path}
            className="dropdown-item"
            onClick={() => {
              setOpen(false);
              onNavigate();
            }}
          >
            {label(item)}
          </RouterNavLink>
        ))}
      </div>
    </li>
  );
};

interface MenuState {
  isOpen: boolean;
  langOpen: boolean;
}

// The menu is server-driven: Config.getHead() are the top-level entries and
// Config.getSubmenus() the dropdown groups, both already privilege-filtered.
class Menu extends Component<{}, MenuState> {
  state: MenuState = { isOpen: false, langOpen: false };
  private unsubscribeI18n?: () => void;

  // Class component, so useT()'s hook isn't available: subscribe manually and
  // force a re-render whenever the locale or a translation resolves.
  componentDidMount() {
    this.unsubscribeI18n = subscribe(() => this.forceUpdate());
    document.addEventListener("click", this.handleDocClick);
  }
  componentWillUnmount() {
    this.unsubscribeI18n?.();
    document.removeEventListener("click", this.handleDocClick);
  }

  // Close the language switcher on any outside click (same pattern as
  // FieldsetList's column-chooser menu).
  handleDocClick = (e: MouseEvent) => {
    if (!this.state.langOpen) return;
    const target = e.target as HTMLElement;
    if (!target.closest(".menu-lang")) this.setState({ langOpen: false });
  };

  toggle = () => this.setState((s) => ({ isOpen: !s.isOpen }));
  // Collapse the mobile menu after navigating, so picking a link doesn't leave
  // the open panel covering the page underneath it.
  close = () => this.setState({ isOpen: false });

  render() {
    const head = Config.getHead();
    const submenus = Config.getSubmenus();
    const authed = Auth.isAuthenticated();
    const avatar = Auth.getAvatar();
    // Profile and login/logout get their own slots on the right, after the
    // language switcher, so they're pulled out of the left-aligned items.
    const profileItem = head.find((i) => i.key === "profile");
    const loginItem = head.find((i) => i.key === "login");
    const leftHead = head.filter((i) => i.key !== "profile" && i.key !== "login");

    const renderHeadItem = (item: MenuItem, idx: number): React.ReactNode => {
      // The login item becomes a Logout button for authenticated users.
      if (item.key === "login" && authed) {
        return (
          <li className="nav-item" key="logout">
            <a
              href="#"
              className="nav-link"
              onClick={(e: React.MouseEvent) => {
                e.preventDefault();
                this.close();
                Auth.logout();
              }}
            >
              <span className="menu-icon-sprite icon-logout" aria-hidden="true" />
              {t("Logout")}
            </a>
          </li>
        );
      }
      return (
        <li className="nav-item" key={"h" + idx}>
          <RouterNavLink
            to={item.path}
            className={({ isActive }: { isActive: boolean }) => `nav-link${isActive ? " active" : ""}`}
            onClick={this.close}
          >
            {label(item)}
          </RouterNavLink>
        </li>
      );
    };

    return (
      <div className="main-menu" style={menuDiv}>
        <nav className="navbar navbar-dark bg-dark navbar-expand-md">
          {/* Brand mark — purely decorative (a plain Link, not NavLink, so it
              never picks up "active" nav styling); the "Home" nav item below
              still covers real/keyboard navigation. */}
          <Link to="/" className="menu-brand" onClick={this.close}>
            <img src="/img/favico_256.png" alt="" className="menu-brand-logo" />
          </Link>

          <button
            type="button"
            className="navbar-toggler"
            onClick={this.toggle}
            aria-label="Toggle navigation"
          >
            <span className="navbar-toggler-icon" />
          </button>
          <div className={`collapse navbar-collapse${this.state.isOpen ? " show" : ""}`}>
            <ul className="navbar-nav">
                {/* Home link — the Home page's href is "" (the root route "/"), so it
                  isn't in the data-driven head list; add it explicitly here. `end`
                  keeps it active only on exactly "/", not on every route. */}
              <li className="nav-item">
                <RouterNavLink
                  to="/"
                  end
                  className={({ isActive }: { isActive: boolean }) => `nav-link${isActive ? " active" : ""}`}
                  onClick={this.close}
                >
                  <span className="menu-icon-sprite icon-home" aria-hidden="true" />
                  {t("Home")}
                </RouterNavLink>
              </li>

              {leftHead.map(renderHeadItem)}

              {Object.keys(submenus).map((title) => {
                const items = submenus[title];
                if (!items || items.length === 0) return null;
                return (
                  <NavDropdown key={"s" + title} title={t(title)} items={items} onNavigate={this.close} />
                );
              })}
            </ul>

            {/* Right-aligned account cluster: language, then avatar + name/role. */}
            <ul className="navbar-nav ml-auto align-items-md-center">
              {/* Not a dropdown menu — the current-locale pill itself expands
                  in place to reveal the other locale(s) directly beneath it,
                  rather than popping a separate floating box. */}
              <li className="nav-item menu-lang-wrap">
                <div className={`menu-lang ${this.state.langOpen ? "menu-lang-open" : ""}`}>
                  <button
                    type="button"
                    className="menu-lang-current"
                    onClick={() => this.setState((s) => ({ langOpen: !s.langOpen }))}
                    aria-expanded={this.state.langOpen}
                    aria-label={t("Language")}
                  >
                    {getLocale().toUpperCase()}
                  </button>
                  <div className="menu-lang-options">
                    {locales
                      .filter((l) => l !== getLocale())
                      .map((l) => (
                        <button
                          type="button"
                          key={l}
                          className="menu-lang-option"
                          onClick={() => {
                            setLocale(l);
                            this.setState({ langOpen: false });
                          }}
                        >
                          {l.toUpperCase()}
                        </button>
                      ))}
                  </div>
                </div>
              </li>

              {authed && profileItem && (
                <li className="nav-item">
                  <RouterNavLink
                    to={profileItem.path}
                    onClick={this.close}
                    className={({ isActive }: { isActive: boolean }) =>
                      `nav-link menu-account${isActive ? " active" : ""}`
                    }
                  >
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
                  </RouterNavLink>
                </li>
              )}

              {loginItem && renderHeadItem(loginItem, -1)}
            </ul>
          </div>
        </nav>
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