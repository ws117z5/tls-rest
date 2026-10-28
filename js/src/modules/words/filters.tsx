import React from "react";
import { FieldsetFilters } from "@engine/fields";
import type { ModuleFiltersProps } from "@engine/controllers/registry";
import useT from "@engine/controllers/useT";
import Auth from "@engine/controllers/auth";

// Adds an "Only mine" checkbox on top of the declared filters (word, user).
// ?own=true isn't a declared column filter — admins see every user's words
// by default and narrow with this param (see go/modules/words/filters.go and
// fieldset_engine.go's buildScopeConditions) — so it can't render through the
// generic FieldsetFilters path and needs this override instead. Non-admins
// are already always scoped to their own words, so the toggle is a no-op for
// them and stays hidden.
const WordsFilters: React.FC<ModuleFiltersProps> = (props) => {
	const t = useT();
	const { values, onChange } = props;
	const ownOnly = values?.own === "true" || values?.own === true;

	return (
		<>
			<FieldsetFilters {...props} />
			{Auth.isAdmin() && (
				<div className="form-check mb-3">
					<input
						type="checkbox"
						className="form-check-input"
						id="words-own-only"
						checked={ownOnly}
						onChange={(e) => onChange("own", e.target.checked ? "true" : "")}
					/>
					<label className="form-check-label" htmlFor="words-own-only">
						{t("Only mine")}
					</label>
				</div>
			)}
		</>
	);
};

export default WordsFilters;
