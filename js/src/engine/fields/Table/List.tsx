import React, { useEffect, useRef, useState } from "react";
import Icon from "@engine/containers/Icon";
import { t } from "@engine/controllers/i18n";
import TableView from "./View";

const TABLE_ICON = "log";

let openId: number | null = null;
let nextId = 0;
const listeners = new Set<() => void>();

const setOpen = (id: number | null) => {
  openId = id;
  listeners.forEach((l) => l());
};

// TYPE_TABLE cell in list mode: a table icon that expands into the full table; opening one collapses whichever was open.
const TableList: React.FC<any> = (props) => {
  const id = useRef(nextId++).current;
  const [, rerender] = useState(0);

  useEffect(() => {
    const l = () => rerender((n) => n + 1);
    listeners.add(l);
    return () => {
      listeners.delete(l);
      if (openId === id) openId = null;
    };
  }, [id]);

  const open = openId === id;
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <button
        type="button"
        className="btn btn-outline-secondary btn-sm"
        title={props.field?.label ? t(props.field.label) : undefined}
        aria-expanded={open}
        onClick={() => setOpen(open ? null : id)}
      >
        <Icon name={TABLE_ICON} />
      </button>
      {open && (
        <div className="mt-2">
          <TableView {...props} />
        </div>
      )}
    </div>
  );
};

export default TableList;
