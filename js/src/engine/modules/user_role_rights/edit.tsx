import React, { useEffect, useState } from "react";
import axios from "axios";
import { Field as LayoutField, useFormLayout } from "@engine/fields/FormLayout";
import { t } from "@engine/controllers/i18n";

// Default edit/create view for the Role Rights module (user_role_rights) —
// registry.ts's fallback also serves this as the create view, since there's
// no separate create.tsx here.
//
// Once both group_id and module have a value, fetches that group's own
// user_group_rights row for the same module and prefills modes/fields/
// filter_fields/special_rights: a one-time starting point the admin then
// narrows down into the role's actual ceiling, not a live binding. module's
// own options are already filtered to that group's rights (see
// modulerights.go's roleModuleOptions), so by the time a module is picked
// there's always a matching group_rights row to copy from.
//
// "fields"/"filter_fields"/"special_rights" are TYPE_TABLE columns with no
// backing SQL, so the generic list/view endpoints never return their stored
// value (see fieldset_engine.shouldIncludeField) — their real display shape
// only exists behind each field's own TableData endpoint
// (/api/modules/user_group_rights/table/{field}), which is what's fetched
// below instead of the raw stored column.

async function findGroupRightsRow(moduleId: string, groupId: string): Promise<{ id: number; modes: number } | null> {
  const res = await axios.get("/user_group_rights", { params: { module: moduleId, page: 1, limit: 200 } });
  const rows: any[] = (res.data && res.data.Data) || [];
  const match = rows.find((r) => String(r.group_id) === String(groupId));
  return match ? { id: match.id, modes: match.modes } : null;
}

async function fetchTableRows(id: number, moduleId: string, field: string): Promise<any[]> {
  const res = await axios.post(`/api/modules/user_group_rights/table/${field}`, { id, module: moduleId });
  return (res.data && res.data.rows) || [];
}

const RoleRightsEdit: React.FC<Record<string, unknown>> = () => {
  const layout = useFormLayout();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<string>("");
  const groupId = layout?.getValue("group_id");
  const moduleId = layout?.getValue("module");

  useEffect(() => {
    if (!layout || !groupId || !moduleId) {
      setNotice("");
      return;
    }
    let cancelled = false;
    setLoading(true);
    setNotice("");
    (async () => {
      try {
        const row = await findGroupRightsRow(moduleId, groupId);
        if (!row) {
          if (!cancelled) setNotice(t("That group has no rights recorded for this module yet."));
          return;
        }
        const [fields, filterFields, specialRights] = await Promise.all([
          fetchTableRows(row.id, moduleId, "fields"),
          fetchTableRows(row.id, moduleId, "filter_fields"),
          fetchTableRows(row.id, moduleId, "special_rights"),
        ]);
        if (cancelled) return;
        layout.onFieldChange("modes", row.modes ?? 0);
        layout.onFieldChange("fields", JSON.stringify(fields));
        layout.onFieldChange("filter_fields", JSON.stringify(filterFields));
        layout.onFieldChange("special_rights", JSON.stringify(specialRights));
      } catch {
        if (!cancelled) setNotice(t("Couldn't load that group's rights."));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [groupId, moduleId]);

  return (
    <div>
      <LayoutField name="group_id" />
      <LayoutField name="role" />
      <LayoutField name="module" />
      {notice && <div className="text-muted small mb-2">{notice}</div>}
      {loading && <div className="text-muted small mb-2">{t("Loading…")}</div>}
      <LayoutField name="modes" />
      <LayoutField name="fields" />
      <LayoutField name="filter_fields" />
      <LayoutField name="special_rights" />
    </div>
  );
};

export default RoleRightsEdit;
