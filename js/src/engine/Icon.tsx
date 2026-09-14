import React from "react";

interface IconProps {
  name: string;
  className?: string;
}

// A small icon cut from the shared sprite sheet (/img/icons_bw.png) — the same
// one Menu.tsx uses for nav icons — reusable in buttons anywhere on the site.
const Icon: React.FC<IconProps> = ({ name, className }) => (
  <span
    className={`menu-icon-sprite icon-${name}${className ? ` ${className}` : ""}`}
    aria-hidden="true"
  />
);

export default Icon;
