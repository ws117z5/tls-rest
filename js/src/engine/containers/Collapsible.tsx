import React, { useState } from "react";
import useT from "@engine/controllers/useT";

interface Props {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}

// Minimal expand/collapse section — no collapsible component existed
// anywhere in this codebase before (see posts/edit.tsx, the first user).
const Collapsible: React.FC<Props> = ({ title, defaultOpen = false, children }) => {
  const [open, setOpen] = useState(defaultOpen);
  const t = useT();

  return (
    <div className="collapsible mb-3">
      <button
        type="button"
        className="btn btn-link p-0 d-flex align-items-center gap-2 text-decoration-none"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <span className={`collapsible-caret${open ? " collapsible-caret-open" : ""}`}>▸</span>
        <span className="fw-semibold">{t(title)}</span>
      </button>
      {open && <div className="collapsible-body mt-2">{children}</div>}
    </div>
  );
};

export default Collapsible;
