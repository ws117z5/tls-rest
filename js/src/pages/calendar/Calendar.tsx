import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import axios from "axios";
import PageComponent from "@engine/containers/PageComponent";
import { t } from "@engine/controllers/i18n";
import AppConfig from "@engine/controllers/Appconfig";
import "@css/calendar.css";

// Calendar page: month/week/day visualization over the events module's own
// data (GET/POST /events, the module's ordinary CRUD endpoint — see
// go/pages/calendar/calendar.go). Creating by click or drag-select posts
// {title, starts_at, duration_minutes, repeat, sync_with_google} and lets the
// server default everything else (scope defaults to "self"); refining
// scope/notification/description is done by following the created event's
// link into the Events module's own edit form, which already has the full
// field set (autocomplete, etc.). Connecting a Google account happens on the
// Profile page, not here (go/engine/pages/profile).

type RepeatOption = "none" | "daily" | "weekly" | "monthly" | "annual";

interface EventRow {
  id: number;
  title: string;
  starts_at: string; // ISO
  duration_minutes: number;
  scope?: string;
  repeat?: RepeatOption;
}

type ViewMode = "month" | "week" | "day";

const HOUR_PX = 48; // px per hour row, week/day views
const START_HOUR = 0;
const END_HOUR = 24;

function startOfDay(d: Date): Date {
  const out = new Date(d);
  out.setHours(0, 0, 0, 0);
  return out;
}

function addDays(d: Date, n: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + n);
  return out;
}

function startOfWeek(d: Date): Date {
  const out = startOfDay(d);
  out.setDate(out.getDate() - out.getDay());
  return out;
}

function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

// rangeForView returns [from, to) covering every day the grid displays.
function rangeForView(view: ViewMode, anchor: Date): [Date, Date] {
  if (view === "day") {
    const from = startOfDay(anchor);
    return [from, addDays(from, 1)];
  }
  if (view === "week") {
    const from = startOfWeek(anchor);
    return [from, addDays(from, 7)];
  }
  const first = startOfMonth(anchor);
  const gridStart = startOfWeek(first);
  return [gridStart, addDays(gridStart, 42)]; // 6 full weeks
}

function fmtISO(d: Date): string {
  return d.toISOString();
}

function sameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

// occursOn reports whether e has an occurrence on day — its own start date,
// or (for a repeating event) any later date matching its pattern. Note: this
// can only expand events the server actually returned, and the from/to list
// filter matches the stored starts_at column literally — a repeating event
// anchored well before the visible range won't be fetched at all until a
// view whose range covers its original starts_at.
function occursOn(e: EventRow, day: Date): boolean {
  const start = new Date(e.starts_at);
  if (sameDay(start, day)) return true;
  if (!e.repeat || e.repeat === "none" || day < startOfDay(start)) return false;
  switch (e.repeat) {
    case "daily":
      return true;
    case "weekly":
      return day.getDay() === start.getDay();
    case "monthly":
      return day.getDate() === start.getDate();
    case "annual":
      return day.getDate() === start.getDate() && day.getMonth() === start.getMonth();
    default:
      return false;
  }
}

async function fetchEvents(from: Date, to: Date): Promise<EventRow[]> {
  const res = await axios.get("/events", { params: { from: fmtISO(from), to: fmtISO(to), page: 1 } });
  return (res.data && res.data.Data) || [];
}

interface SelectOption {
  value: string;
  name: string;
}

// ScopeMeta is the current viewer's own role-resolved scope controls — "who
// can see this event" (events.go's scopeOptions/targetGroupOptions): an
// empty scopeOptions means the viewer never sees a scope control at all
// (always "self"), same convention as the generic Events form.
interface ScopeMeta {
  scopeOptions: SelectOption[];
  groupOptions: SelectOption[];
}

async function fetchScopeMeta(): Promise<ScopeMeta> {
  const res = await axios.post("/api/modules/events/fieldset", {});
  const fields: any[] = (res.data && res.data.fields) || [];
  const find = (name: string) => fields.find((f) => f.name === name);
  const toOptions = (raw: any[] | undefined): SelectOption[] =>
    (raw || []).map((o) => ({ value: String(o.value), name: o.name }));
  return {
    scopeOptions: toOptions(find("scope")?.options?.options),
    groupOptions: toOptions(find("target_group_id")?.options?.options),
  };
}

async function searchUsers(input: string): Promise<SelectOption[]> {
  const res = await axios.post("/api/modules/events/autocomplete/target_user_id", { input, values: {} });
  const opts: any[] = (res.data && res.data.options) || [];
  return opts.map((o) => ({ value: String(o.value), name: o.label }));
}

interface CreateEventArgs {
  title: string;
  startsAt: Date;
  durationMinutes: number;
  repeat: RepeatOption;
  syncWithGoogle: boolean;
  scope?: string;
  targetGroupId?: string;
  targetUserId?: string;
}

async function createEvent(args: CreateEventArgs): Promise<void> {
  const body: Record<string, unknown> = {
    title: args.title,
    starts_at: fmtISO(args.startsAt),
    duration_minutes: Math.max(5, Math.round(args.durationMinutes)),
    repeat: args.repeat,
    sync_with_google: args.syncWithGoogle,
  };
  if (args.scope) body.scope = args.scope;
  if (args.targetGroupId) body.target_group_id = args.targetGroupId;
  if (args.targetUserId) body.target_user_id = args.targetUserId;
  await axios.post("/events", body);
}

// --- quick-create popover --------------------------------------------------

interface DraftRange {
  start: Date;
  end: Date; // for a plain click, end === start + 60min
}

const QuickCreate: React.FC<{
  draft: DraftRange;
  scopeMeta: ScopeMeta;
  onCancel: () => void;
  onCreated: () => void;
}> = ({ draft, scopeMeta, onCancel, onCreated }) => {
  const [title, setTitle] = useState("");
  const [repeat, setRepeat] = useState<RepeatOption>("none");
  const [syncWithGoogle, setSyncWithGoogle] = useState(AppConfig.syncWithGoogleDefault());
  const [saving, setSaving] = useState(false);

  // Scope controls only exist at all for a viewer scopeMeta.scopeOptions
  // actually offers something beyond "self" (events.go's scopeOptions) —
  // "everyone else should not have scope".
  const hasScopeControl = scopeMeta.scopeOptions.length > 1;
  const [scope, setScope] = useState<string>("self");
  const [targetGroupId, setTargetGroupId] = useState<string>(scopeMeta.groupOptions[0]?.value || "");
  // scopeMeta is fetched once by the parent and normally resolves well before
  // a draft exists, but guard the rare case this mounted first anyway.
  useEffect(() => {
    if (!targetGroupId && scopeMeta.groupOptions[0]) setTargetGroupId(scopeMeta.groupOptions[0].value);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scopeMeta.groupOptions]);
  const [userQuery, setUserQuery] = useState("");
  const [userResults, setUserResults] = useState<SelectOption[]>([]);
  const [targetUser, setTargetUser] = useState<SelectOption | null>(null);
  const userSearchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const onUserQueryChange = (value: string) => {
    setUserQuery(value);
    setTargetUser(null);
    if (userSearchTimer.current) clearTimeout(userSearchTimer.current);
    if (!value.trim()) {
      setUserResults([]);
      return;
    }
    userSearchTimer.current = setTimeout(() => {
      searchUsers(value).then(setUserResults).catch(() => setUserResults([]));
    }, 200);
  };

  const submit = async () => {
    if (!title.trim()) return;
    if (scope === "user" && !targetUser) return;
    setSaving(true);
    try {
      const minutes = Math.max(5, (draft.end.getTime() - draft.start.getTime()) / 60000);
      await createEvent({
        title: title.trim(),
        startsAt: draft.start,
        durationMinutes: minutes,
        repeat,
        syncWithGoogle,
        scope: hasScopeControl ? scope : undefined,
        targetGroupId: scope === "group" ? targetGroupId : undefined,
        targetUserId: scope === "user" ? targetUser?.value : undefined,
      });
      onCreated();
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card module-page-card calendar-side-panel">
      <div className="card-body">
        <h5 className="card-title h6 text-uppercase text-muted mb-3">{t("New event")}</h5>
        <div className="small text-muted mb-2">
          {draft.start.toLocaleString()} &ndash; {draft.end.toLocaleTimeString()}
        </div>
        <input
          className="form-control form-control-sm mb-2"
          placeholder={t("Event title")}
          value={title}
          autoFocus
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
            if (e.key === "Escape") onCancel();
          }}
        />
        <select
          className="form-control form-control-sm mb-2"
          value={repeat}
          onChange={(e) => setRepeat(e.target.value as RepeatOption)}
        >
          <option value="none">{t("Does not repeat")}</option>
          <option value="daily">{t("Daily")}</option>
          <option value="weekly">{t("Weekly")}</option>
          <option value="monthly">{t("Monthly")}</option>
          <option value="annual">{t("Annually")}</option>
        </select>
        {hasScopeControl && (
          <select
            className="form-control form-control-sm mb-2"
            value={scope}
            onChange={(e) => setScope(e.target.value)}
          >
            {scopeMeta.scopeOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.name}
              </option>
            ))}
          </select>
        )}
        {scope === "group" && (
          <select
            className="form-control form-control-sm mb-2"
            value={targetGroupId}
            onChange={(e) => setTargetGroupId(e.target.value)}
          >
            {scopeMeta.groupOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.name}
              </option>
            ))}
          </select>
        )}
        {scope === "user" && (
          <div className="mb-2" style={{ position: "relative" }}>
            <input
              className="form-control form-control-sm"
              placeholder={t("Search user by name…")}
              value={targetUser ? targetUser.name : userQuery}
              onChange={(e) => onUserQueryChange(e.target.value)}
            />
            {!targetUser && userResults.length > 0 && (
              <ul
                className="list-group"
                style={{ position: "absolute", zIndex: 30, width: "100%", maxHeight: 160, overflowY: "auto" }}
              >
                {userResults.map((o) => (
                  <li
                    key={o.value}
                    className="list-group-item list-group-item-action"
                    style={{ cursor: "pointer", padding: "4px 10px" }}
                    onMouseDown={() => {
                      setTargetUser(o);
                      setUserResults([]);
                    }}
                  >
                    {o.name}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
        <label className="d-flex align-items-center gap-2 mb-2 small">
          <input type="checkbox" checked={syncWithGoogle} onChange={(e) => setSyncWithGoogle(e.target.checked)} />
          {t("Sync with Google Calendar")}
        </label>
        <div className="d-flex gap-2">
          <button
            className="btn btn-sm btn-primary"
            disabled={saving || !title.trim() || (scope === "user" && !targetUser)}
            onClick={submit}
          >
            {t("Create")}
          </button>
          <button className="btn btn-sm btn-outline-secondary" onClick={onCancel}>
            {t("Cancel")}
          </button>
        </div>
      </div>
    </div>
  );
};

// --- month view -------------------------------------------------------------

const MonthGrid: React.FC<{
  anchor: Date;
  events: EventRow[];
  onPick: (range: DraftRange) => void;
}> = ({ anchor, events, onPick }) => {
  const [gridStart] = rangeForView("month", anchor);
  const days = useMemo(() => Array.from({ length: 42 }, (_, i) => addDays(gridStart, i)), [gridStart]);
  const today = new Date();

  const eventsOn = (day: Date) => events.filter((e) => occursOn(e, day));

  return (
    <div className="calendar-month-grid">
      {days.map((day, i) => {
        const inMonth = day.getMonth() === anchor.getMonth();
        return (
          <div
            key={i}
            className={`calendar-day-cell${inMonth ? "" : " calendar-day-outside"}`}
            onClick={() => {
              const start = new Date(day);
              start.setHours(9, 0, 0, 0);
              onPick({ start, end: new Date(start.getTime() + 60 * 60000) });
            }}
          >
            <div className={`calendar-day-number${sameDay(day, today) ? " calendar-day-today" : ""}`}>
              {day.getDate()}
            </div>
            {eventsOn(day).map((e) => (
              <a
                key={e.id}
                href={`/events/${e.id}/edit`}
                className="calendar-event-chip"
                onClick={(ev) => ev.stopPropagation()}
              >
                {e.title}
              </a>
            ))}
          </div>
        );
      })}
    </div>
  );
};

// --- week/day view -----------------------------------------------------------

// yToMinutes converts a pointer's offset within the grid body into minutes
// since START_HOUR, snapped to 15-minute steps.
function yToMinutes(offsetY: number): number {
  const raw = (offsetY / HOUR_PX) * 60;
  return Math.max(0, Math.round(raw / 15) * 15);
}

const TimeGrid: React.FC<{
  days: Date[];
  events: EventRow[];
  onPick: (range: DraftRange) => void;
}> = ({ days, events, onPick }) => {
  const [dragCol, setDragCol] = useState<number | null>(null);
  const [dragStartMin, setDragStartMin] = useState<number | null>(null);
  const [dragEndMin, setDragEndMin] = useState<number | null>(null);

  const hours = useMemo(() => Array.from({ length: END_HOUR - START_HOUR }, (_, i) => START_HOUR + i), []);

  const onMouseDown = (col: number, e: React.MouseEvent<HTMLDivElement>) => {
    const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect();
    const min = yToMinutes(e.clientY - rect.top);
    setDragCol(col);
    setDragStartMin(min);
    setDragEndMin(min);
  };

  const onMouseMove = (col: number, e: React.MouseEvent<HTMLDivElement>) => {
    if (dragCol !== col || dragStartMin === null) return;
    const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect();
    setDragEndMin(yToMinutes(e.clientY - rect.top));
  };

  const onMouseUp = (col: number) => {
    if (dragCol !== col || dragStartMin === null) return;
    const lo = Math.min(dragStartMin, dragEndMin ?? dragStartMin);
    const hi = Math.max(dragStartMin, dragEndMin ?? dragStartMin);
    const start = new Date(days[col]);
    start.setHours(0, lo, 0, 0);
    const end = new Date(days[col]);
    // A plain click (no meaningful drag) defaults to a 60-minute event.
    end.setHours(0, hi > lo + 10 ? hi : lo + 60, 0, 0);
    onPick({ start, end });
    setDragCol(null);
    setDragStartMin(null);
    setDragEndMin(null);
  };

  const eventsOn = (day: Date) => events.filter((e) => occursOn(e, day));
  const today = new Date();

  return (
    <div className="calendar-time-grid">
      <div className="calendar-time-header">
        <div className="calendar-hour-gutter" />
        {days.map((day, col) => (
          <div
            key={col}
            className={`calendar-time-header-cell${sameDay(day, today) ? " calendar-day-today" : ""}`}
          >
            {day.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}
          </div>
        ))}
      </div>
      <div className="calendar-time-body">
        <div className="calendar-hour-gutter">
          {hours.map((h) => (
            <div key={h} className="calendar-hour-row calendar-hour-label" style={{ height: HOUR_PX }}>
              {h}:00
            </div>
          ))}
        </div>
        {days.map((day, col) => (
          <div
            key={col}
            className="calendar-time-col"
            onMouseDown={(e) => onMouseDown(col, e)}
            onMouseMove={(e) => onMouseMove(col, e)}
            onMouseUp={() => onMouseUp(col)}
          >
            {hours.map((h) => (
              <div key={h} className="calendar-hour-row" style={{ height: HOUR_PX }} />
            ))}
            {dragCol === col && dragStartMin !== null && dragEndMin !== null && (
              <div
                className="calendar-drag-box"
                style={{
                  top: (Math.min(dragStartMin, dragEndMin) / 60) * HOUR_PX,
                  height: Math.max(4, (Math.abs(dragEndMin - dragStartMin) / 60) * HOUR_PX),
                }}
              />
            )}
            {eventsOn(day).map((e) => {
              const s = new Date(e.starts_at);
              const top = ((s.getHours() * 60 + s.getMinutes()) / 60) * HOUR_PX;
              const height = Math.max(16, (e.duration_minutes / 60) * HOUR_PX);
              return (
                <a key={e.id} href={`/events/${e.id}/edit`} className="calendar-event-block" style={{ top, height }}>
                  {e.title}
                </a>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
};

// --- top-level grid -----------------------------------------------------------

const CalendarGrid: React.FC = () => {
  const [view, setView] = useState<ViewMode>("month");
  const [anchor, setAnchor] = useState<Date>(new Date());
  const [events, setEvents] = useState<EventRow[]>([]);
  const [draft, setDraft] = useState<DraftRange | null>(null);
  const [scopeMeta, setScopeMeta] = useState<ScopeMeta>({ scopeOptions: [], groupOptions: [] });

  const [from, to] = useMemo(() => rangeForView(view, anchor), [view, anchor]);

  const reload = useCallback(() => {
    fetchEvents(from, to)
      .then(setEvents)
      .catch(() => setEvents([]));
  }, [from, to]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => {
    fetchScopeMeta()
      .then(setScopeMeta)
      .catch(() => setScopeMeta({ scopeOptions: [], groupOptions: [] }));
  }, []);

  const step = (dir: 1 | -1) => {
    if (view === "day") setAnchor(addDays(anchor, dir));
    else if (view === "week") setAnchor(addDays(anchor, 7 * dir));
    else setAnchor(new Date(anchor.getFullYear(), anchor.getMonth() + dir, 1));
  };

  const label =
    view === "month"
      ? anchor.toLocaleDateString(undefined, { month: "long", year: "numeric" })
      : view === "week"
        ? `${startOfWeek(anchor).toLocaleDateString()} – ${addDays(startOfWeek(anchor), 6).toLocaleDateString()}`
        : anchor.toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric", year: "numeric" });

  const weekDays = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(startOfWeek(anchor), i)), [anchor]);

  return (
    <div className="container-fluid module-page pt-4">
      <div className="module-page-header d-flex justify-content-between align-items-center mb-3">
        <h1 className="h4 mb-0">{t("Calendar")}</h1>
      </div>
      <div className="calendar-layout">
        <div className="card module-page-card calendar-grid-card">
          <div className="card-body">
            <div className="d-flex align-items-center gap-2 mb-3 flex-wrap">
              <button className="btn btn-sm btn-outline-secondary" onClick={() => step(-1)}>
                &lsaquo;
              </button>
              <button className="btn btn-sm btn-outline-secondary" onClick={() => setAnchor(new Date())}>
                {t("Today")}
              </button>
              <button className="btn btn-sm btn-outline-secondary" onClick={() => step(1)}>
                &rsaquo;
              </button>
              <div className="fw-semibold ml-2">{label}</div>
              <div className="ml-auto btn-group">
                {(["month", "week", "day"] as ViewMode[]).map((v) => (
                  <button
                    key={v}
                    className={`btn btn-sm ${view === v ? "btn-primary" : "btn-outline-secondary"}`}
                    onClick={() => setView(v)}
                  >
                    {t(v.charAt(0).toUpperCase() + v.slice(1))}
                  </button>
                ))}
              </div>
            </div>

            {view === "month" && <MonthGrid anchor={anchor} events={events} onPick={setDraft} />}
            {view === "week" && <TimeGrid days={weekDays} events={events} onPick={setDraft} />}
            {view === "day" && <TimeGrid days={[startOfDay(anchor)]} events={events} onPick={setDraft} />}
          </div>
        </div>

        {draft && (
          <QuickCreate
            draft={draft}
            scopeMeta={scopeMeta}
            onCancel={() => setDraft(null)}
            onCreated={() => {
              setDraft(null);
              reload();
            }}
          />
        )}
      </div>
    </div>
  );
};

export default class Calendar extends PageComponent<{}, {}> {
  protected isPage = true;
  protected href = "calendar";
  protected title = "Calendar";
  // No submenu: top-level, like the messages page — Menu.tsx special-cases
  // this key to render it with the due-reminders badge.
  protected submenu = "";
  protected icon = "calendar";
  protected requiresAuth = true;
  protected loadsData = false;

  render() {
    return <CalendarGrid />;
  }
}
