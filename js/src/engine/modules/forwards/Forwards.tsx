import React from "react";
import { t } from "@engine/controllers/i18n";

// Read-only forward count (go/engine/modules/forwards) — the web app has no
// P2P chat system to forward INTO, so unlike Views this never POSTs; the
// Android app is the only thing that records a forward.
interface Props {
  count: number;
}

const Forwards: React.FC<Props> = ({ count }) => (
  <span className="text-muted small">
    {count} {t("forwards")}
  </span>
);

export default Forwards;
