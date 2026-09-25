import React from "react";

interface IconProps {
  name: string;
  className?: string;
  // Multiplies the icon's base zoom (see css/icon.css's --icon-scale); 1 = default size.
  scale?: number;
  align?: React.CSSProperties["verticalAlign"];
  animate?: "spin" | "pulse";
}

// A small icon cut from the shared sprite sheet (/img/icons_bw.png — see
// css/icon.css), reusable in buttons/menus/badges anywhere on the site.
const Icon: React.FC<IconProps> = ({ name, className, scale, align, animate }) => {
  const classes = ["menu-icon-sprite", `icon-${name}`];
  if (animate) classes.push(`icon-${animate}`);
  if (className) classes.push(className);
  const style: React.CSSProperties = {};
  if (scale !== undefined) (style as any)["--icon-scale"] = scale;
  if (align !== undefined) style.verticalAlign = align;
  return (
    <span
      className={classes.join(" ")}
      style={Object.keys(style).length ? style : undefined}
      aria-hidden="true"
    />
  );
};

export default Icon;
