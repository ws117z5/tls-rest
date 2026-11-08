import React, { useRef, useState } from "react";
import axios from "axios";
import { FieldsetProvider, FieldsetForm, MODES } from "@engine/fields";
import type { ModuleViewProps } from "@engine/controllers/registry";
import useT from "@engine/controllers/useT";

// Debounced so a lookup fires once typing actually pauses, not per keystroke
// — translate.googleapis.com is a third-party call, not our own fast autocomplete.
const LOOKUP_DEBOUNCE_MS = 500;

// Mirrors words-android's own add-word flow (LanguageDetector + GoogleTranslator
// in WordsViewModel.addWord) but fetches both language AND translations up
// front, since there's no on-device ML Kit here to call just for language —
// one server-side lookup (go/modules/words/lookup.go) covers both. Android only
// ever fetches translations live during practice, never persists them; this
// create screen persists them via POST /words/{id}/translations right after
// the word itself is created, so the admin-entered word starts with real data.
//
// The fieldset's own "translations" field is virtual (SQL-computed from
// word_translations, read-only, not submitted) — rendering it generically
// would just show a second, always-empty "Translations" box on create (there's
// no row to compute from yet). fieldRenderer below swaps it, in place, for
// this component's own editable JSON textarea, which is what actually ends up
// submitted via POST /words/{id}/translations.
const WordsCreate: React.FC<ModuleViewProps> = ({ module, navigate }) => {
	const t = useT();
	// Bumped (new reference) only when a lookup actually changes the language,
	// so FieldsetForm re-seeds just that field without resetting on every
	// keystroke — see FieldsetForm's componentDidUpdate.
	const [seed, setSeed] = useState<Record<string, any>>({});
	const [translationsText, setTranslationsText] = useState("");
	const [looking, setLooking] = useState(false);
	const [lookupError, setLookupError] = useState("");
	const [saving, setSaving] = useState(false);
	const formRef = useRef<Record<string, any>>({});
	const latestWord = useRef("");
	const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

	const runLookup = (word: string) => {
		if (!word) {
			setTranslationsText("");
			return;
		}
		setLooking(true);
		setLookupError("");
		axios
			.get("/words/lookup", { params: { word } })
			.then((res) => {
				if (latestWord.current !== word) return; // superseded by a later edit
				setTranslationsText(JSON.stringify(res.data?.translations || [], null, 2));
				const lang = res.data?.language;
				if (lang && lang !== "und") {
					formRef.current = { ...formRef.current, language: lang };
					setSeed(formRef.current);
				}
			})
			.catch((e) => {
				if (latestWord.current !== word) return;
				// Best-effort otherwise — leave whatever's already there — but
				// surface a rate limit specifically, since it's the one failure
				// mode worth knowing about rather than silently retrying
				// (translate.googleapis.com is free/unofficial, no published
				// quota — see lookup.go).
				if (e?.response?.status === 429) {
					setLookupError(t("Translation lookup is rate-limited right now — try again in a minute."));
				}
			})
			.finally(() => setLooking(false));
	};

	const onChange = (field: string, value: any) => {
		formRef.current = { ...formRef.current, [field]: value };
		if (field === "word") {
			const w = String(value || "").trim();
			latestWord.current = w;
			if (timer.current) clearTimeout(timer.current);
			timer.current = setTimeout(() => runLookup(w), LOOKUP_DEBOUNCE_MS);
		}
	};

	// Malformed JSON just means nothing gets saved here — not a hard submit failure.
	const parseTranslations = (): string[] => {
		try {
			const parsed = JSON.parse(translationsText);
			if (Array.isArray(parsed)) return parsed.filter((v) => typeof v === "string" && v.trim());
		} catch {
			// ignore
		}
		return [];
	};

	const onSubmit = async (form: Record<string, any>) => {
		setSaving(true);
		try {
			const res = await axios.post("/words", form);
			const id = res?.data?.id;
			const translations = parseTranslations();
			if (id != null && translations.length > 0) {
				await axios.post(`/words/${id}/translations`, { translations });
			}
			if (id != null && navigate) navigate(`/${module}/${id}`);
		} catch (e) {
			console.error("Failed to create word:", e);
		} finally {
			setSaving(false);
		}
	};

	return (
		<FieldsetProvider module={module} mode={MODES.CREATE}>
			<FieldsetForm
				mode={MODES.CREATE}
				data={seed}
				onChange={onChange}
				onSubmit={onSubmit}
				disabled={saving}
				fieldRenderer={(fields, renderField) => (
					<>
						{fields.map((field: any) =>
							field.name === "translations" ? (
								<div key="translations" className="fieldset-row d-flex gap-3 mb-3 align-items-start">
									<div className="fieldset-cell fieldset-cell-desc" style={{ flex: "0 0 220px", maxWidth: 220 }}>
										<div className="fw-semibold">{t("Translations")}</div>
										<div className="text-muted small">
											{looking ? t("Looking up…") : t("Fetched automatically — edit freely before saving")}
										</div>
									</div>
									<div className="fieldset-cell fieldset-cell-value" style={{ flex: "1 1 auto", maxWidth: "30rem" }}>
										<textarea
											className="form-control"
											rows={5}
											value={translationsText}
											onChange={(e) => setTranslationsText(e.target.value)}
											placeholder='["translation 1", "translation 2"]'
										/>
										{lookupError && <div className="text-danger small mt-1">{lookupError}</div>}
									</div>
								</div>
							) : (
								<React.Fragment key={field.name}>{renderField(field)}</React.Fragment>
							),
						)}
					</>
				)}
			/>
		</FieldsetProvider>
	);
};

export default WordsCreate;
