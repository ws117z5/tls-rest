import React, { useEffect, useState } from "react";
import { ModuleViewProps } from "@engine/controllers/registry";
import { FieldsetForm, MODES } from "@engine/fields";
import Collapsible from "@engine/containers/Collapsible";

// The four plain checkboxes (go/modules/posts/posts.go's fieldset()) —
// promoted above the three "subtable" (sharing-list) fields below, inside a
// single collapsed-by-default section. See .claude's page/frontend rule
// doesn't apply here (this overrides a module, not a PageAbstract).
const VISIBILITY_CHECKBOXES = ["visible_anyone", "visible_friends", "visible_subscribers", "visible_to_blocked"];
const VISIBILITY_TABLES = ["visible_users", "visible_groups", "hide_users"];
const VISIBILITY_FIELDS = new Set([...VISIBILITY_CHECKBOXES, ...VISIBILITY_TABLES]);

const STORAGE_KEY = "post-visibility-defaults";

function loadStoredDefaults(): Record<string, any> | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function saveStoredDefault(field: string, value: any) {
  try {
    const saved = loadStoredDefaults() || {};
    saved[field] = value;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(saved));
  } catch {
    // Private browsing / storage disabled — the form still works, just without persistence.
  }
}

// Shared by posts/Edit.tsx and posts/Create.tsx (the registry needs two
// separate files, but the behavior — and the localStorage seeding, which
// only actually applies in "create" mode — is identical either way).
const PostVisibilityForm: React.FC<ModuleViewProps> = ({ mode, record, submit }) => {
  const isNew = mode === "create";
  const [data, setData] = useState<Record<string, any>>(record || {});

  useEffect(() => {
    if (!isNew) return;
    const stored = loadStoredDefaults();
    setData((prev) => ({
      ...prev,
      visible_friends: true,
      visible_subscribers: true,
      ...(stored || {}),
    }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleChange = (field: string, value: any) => {
    if (isNew && VISIBILITY_FIELDS.has(field)) saveStoredDefault(field, value);
  };

  return (
    <FieldsetForm
      mode={isNew ? MODES.CREATE : MODES.EDIT}
      data={data}
      onSubmit={submit}
      onChange={handleChange}
      fieldRenderer={(fields, renderField) => {
        const otherFields = fields.filter((f: any) => !VISIBILITY_FIELDS.has(f.name));
        const checkboxFields = fields.filter((f: any) => VISIBILITY_CHECKBOXES.includes(f.name));
        const tableFields = fields.filter((f: any) => VISIBILITY_TABLES.includes(f.name));
        return (
          <>
            {otherFields.map(renderField)}
            <Collapsible title="Visibility">
              {checkboxFields.map(renderField)}
              {tableFields.map(renderField)}
            </Collapsible>
          </>
        );
      }}
    />
  );
};

export default PostVisibilityForm;
