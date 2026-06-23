// @hijau/web — framework-agnostic in-context SDK core.
//
// This is the piece a developer drops into their running app. It holds the
// translation records for the current language, exposes `t(key)` to render a
// string, and — when running in `edit` mode — appends the zero-width marker
// codec to each rendered string so the in-context editor (@hijau/incontext)
// can scan the DOM, decode a clicked node back to its translation id, and save
// an edit back. It also runs a tiny event bus so edits re-render live.
//
// No DOM, no framework, no network here — those live in @hijau/incontext and
// the framework bindings. Keeping the core pure makes it unit-testable under
// `bun test` and reusable from Svelte, React, or vanilla JS alike.

import { wrap } from '@hijau/i18n';

/** A single translatable string for the active language. `subId` is the DB
 *  `sub_id` (a compact per-project integer) that the marker codec encodes. */
export interface TranslationRecord {
	subId: number;
	key: string;
	text: string;
}

/**
 * `production` renders plain strings (zero overhead, no markers, safe to ship).
 * `edit` wraps every rendered string with its marker so the overlay editor can
 * locate it. Mode can flip at runtime (e.g. when a developer unlocks editing).
 */
export type HijauMode = 'production' | 'edit';

export type HijauEvent =
	| { type: 'records.loaded'; count: number }
	| { type: 'mode.changed'; mode: HijauMode }
	| { type: 'translation.updated'; subId: number; key: string; text: string };

export type HijauListener = (event: HijauEvent) => void;

export interface HijauConfig {
	/** BCP-47 tag of the language these records are for. */
	language: string;
	/** Static records, typically emitted by `hijau pull --with-ids`. */
	records?: TranslationRecord[];
	/** Defaults to `production`. */
	mode?: HijauMode;
	/** API base + project id — required only for live edit/save-back. */
	apiUrl?: string;
	projectId?: string;
}

export interface Hijau {
	readonly language: string;
	readonly apiUrl?: string;
	readonly projectId?: string;
	mode: HijauMode;

	/** Render a key. Returns `fallback ?? key` if unknown. In `edit` mode the
	 *  result carries an invisible marker encoding the record's `subId`. */
	t(key: string, fallback?: string): string;

	getRecord(key: string): TranslationRecord | undefined;
	getRecordBySubId(subId: number): TranslationRecord | undefined;

	/** Replace the record set (e.g. after switching language). */
	setRecords(records: TranslationRecord[]): void;
	setMode(mode: HijauMode): void;

	/** Apply an edit locally and notify listeners so the UI re-renders. The
	 *  network save-back is the editor's job; this keeps the store in sync. */
	applyUpdate(subId: number, text: string): void;

	/** Subscribe to events; returns an unsubscribe function. */
	on(listener: HijauListener): () => void;
}

export function createHijau(config: HijauConfig): Hijau {
	let mode: HijauMode = config.mode ?? 'production';
	const byKey = new Map<string, TranslationRecord>();
	const bySubId = new Map<number, TranslationRecord>();
	const listeners = new Set<HijauListener>();

	function index(records: TranslationRecord[]): void {
		byKey.clear();
		bySubId.clear();
		for (const r of records) {
			byKey.set(r.key, r);
			bySubId.set(r.subId, r);
		}
	}

	function emit(event: HijauEvent): void {
		for (const l of listeners) l(event);
	}

	index(config.records ?? []);

	const hijau: Hijau = {
		language: config.language,
		apiUrl: config.apiUrl,
		projectId: config.projectId,

		get mode() {
			return mode;
		},
		set mode(next: HijauMode) {
			this.setMode(next);
		},

		t(key, fallback) {
			const record = byKey.get(key);
			const text = record?.text ?? fallback ?? key;
			// Only marked records can be edited in place; unknown keys render plain.
			if (mode === 'edit' && record) return wrap(text, record.subId);
			return text;
		},

		getRecord(key) {
			return byKey.get(key);
		},

		getRecordBySubId(subId) {
			return bySubId.get(subId);
		},

		setRecords(records) {
			index(records);
			emit({ type: 'records.loaded', count: records.length });
		},

		setMode(next) {
			if (next === mode) return;
			mode = next;
			emit({ type: 'mode.changed', mode });
		},

		applyUpdate(subId, text) {
			const record = bySubId.get(subId);
			if (!record) return;
			record.text = text;
			emit({ type: 'translation.updated', subId, key: record.key, text });
		},

		on(listener) {
			listeners.add(listener);
			return () => listeners.delete(listener);
		}
	};

	return hijau;
}
