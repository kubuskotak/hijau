// @hijau/incontext — turn any running app into an in-context translation
// editor. Hold the modifier (Alt/Option by default) to reveal translatable
// strings; click one to edit it in an isolated overlay and save back.
//
// It finds editable strings by decoding the zero-width marker that @hijau/web
// appends to each rendered string in `edit` mode — so it works with no
// data-attributes or framework cooperation. Save-back hits the REST API when
// configured, and always calls hijau.applyUpdate() so bindings re-render live.
// With no apiUrl it runs fully offline against the SDK's in-memory records.

import { decode } from '@hijau/i18n';
import type { Hijau } from '@hijau/web';
import { Overlay } from './overlay';
import type { EditContext, InContextOptions, Modifier } from './types';

export type { EditContext, InContextOptions, Modifier } from './types';

const MOD_FLAG: Record<Modifier, 'altKey' | 'ctrlKey' | 'shiftKey' | 'metaKey'> = {
	Alt: 'altKey',
	Control: 'ctrlKey',
	Shift: 'shiftKey',
	Meta: 'metaKey'
};

// Duck-typed so this stays free of DOM globals (Element/Node) — testable under
// bun and safe to call from any context.
type Elementish = { textContent: string | null; parentElement: Elementish | null };

/** Walk up from a node to the nearest element whose text decodes to a sub_id. */
export function markedElementAt(node: EventTarget | null): { el: Element; subId: number } | null {
	let el = node as Elementish | null;
	for (let depth = 0; el && depth < 8; depth++, el = el.parentElement) {
		const subId = decode(el.textContent ?? '');
		if (subId !== null) return { el: el as unknown as Element, subId };
	}
	return null;
}

export function enableInContext(hijau: Hijau, options: InContextOptions = {}): () => void {
	// SSR / non-DOM (e.g. bun test) — no-op so importing is always safe.
	if (typeof document === 'undefined' || typeof window === 'undefined') {
		return () => {};
	}

	const apiUrl = (options.apiUrl ?? hijau.apiUrl ?? '').replace(/\/$/, '');
	const projectId = options.projectId ?? hijau.projectId ?? '';
	const modFlag = MOD_FLAG[options.modifier ?? 'Alt'];
	const overlay = new Overlay();
	overlay.mount();

	const prevMode = hijau.mode;
	hijau.setMode('edit'); // ensure rendered strings carry markers

	let active = false; // modifier currently held
	let hovered: Element | null = null;
	let writeToken: string | null = null; // minted by unlock(); short-lived, user-bound

	// Reads (and the unlock call itself) use the embedded read-only editor token.
	function readHeaders(): Record<string, string> {
		return options.token ? { Authorization: `Bearer ${options.token}` } : {};
	}

	function needsUnlock(message: string): Error {
		const e = new Error(message) as Error & { needsUnlock?: boolean };
		e.needsUnlock = true;
		return e;
	}

	async function unlock(email: string, password: string): Promise<void> {
		const res = await fetch(`${apiUrl}/projects/${projectId}/editor/unlock`, {
			method: 'POST',
			credentials: 'include',
			headers: { ...readHeaders(), 'Content-Type': 'application/json' },
			body: JSON.stringify({ email, password })
		});
		if (!res.ok) {
			let msg = `Sign-in failed (HTTP ${res.status})`;
			try {
				const body = await res.json();
				if (body?.error?.message) msg = body.error.message;
			} catch {
				/* keep default */
			}
			throw new Error(msg);
		}
		const data = (await res.json()) as { token: string };
		writeToken = data.token;
	}

	async function resolveContext(subId: number): Promise<EditContext> {
		if (apiUrl && projectId) {
			const res = await fetch(`${apiUrl}/projects/${projectId}/translations/by-subid/${subId}`, {
				credentials: 'include',
				headers: readHeaders()
			});
			if (!res.ok) throw new Error(`Could not load translation (HTTP ${res.status})`);
			return (await res.json()) as EditContext;
		}
		// Offline: synthesise a minimal context from the in-memory record.
		const rec = hijau.getRecordBySubId(subId);
		if (!rec) throw new Error('Unknown translation');
		return {
			translation: { id: '', subId, text: rec.text, state: 'editing', languageId: '', version: 0 },
			key: { id: '', name: rec.key, description: '', namespaceId: '' },
			language: { tag: hijau.language, name: hijau.language, isRtl: false },
			sourceText: ''
		};
	}

	async function save(ctx: EditContext, text: string): Promise<void> {
		if (apiUrl && projectId && ctx.key.id) {
			// Writing requires an unlocked, user-bound token — the read-only editor
			// token can't write. Ask the overlay to collect credentials if we
			// don't have one yet.
			if (!writeToken) throw needsUnlock('Sign in to save your changes');

			const res = await fetch(
				`${apiUrl}/projects/${projectId}/keys/${ctx.key.id}/translations/${ctx.language.tag}`,
				{
					method: 'PUT',
					credentials: 'include',
					headers: { Authorization: `Bearer ${writeToken}`, 'Content-Type': 'application/json' },
					body: JSON.stringify({ text })
				}
			);
			if (res.status === 401 || res.status === 403) {
				writeToken = null; // expired or revoked — re-unlock
				throw needsUnlock('Your editing session expired — sign in again');
			}
			if (!res.ok) {
				let msg = `Save failed (HTTP ${res.status})`;
				try {
					const body = await res.json();
					if (body?.error?.message) msg = body.error.message;
				} catch {
					/* keep default */
				}
				throw new Error(msg);
			}
		}
		hijau.applyUpdate(ctx.translation.subId, text);
		options.onSave?.(ctx, text);
	}

	// Capture the page to a PNG and upload it, tagging a region for the element
	// being edited so translators see the string in context.
	async function capture(subId: number, el: Element, name: string): Promise<void> {
		if (!apiUrl || !projectId) throw new Error('Screenshots need a live connection (set apiUrl + projectId)');
		if (!writeToken) throw needsUnlock('Save once to sign in, then capture');
		const { toPng } = await import('html-to-image');
		const dataUrl = await toPng(document.body, {
			cacheBust: true,
			// keep our own overlay out of the shot
			filter: (node: HTMLElement) => !node.hasAttribute?.('data-hijau-overlay')
		});
		const r = el.getBoundingClientRect();
		const res = await fetch(`${apiUrl}/projects/${projectId}/screenshots`, {
			method: 'POST',
			credentials: 'include',
			headers: { Authorization: `Bearer ${writeToken}`, 'Content-Type': 'application/json' },
			body: JSON.stringify({
				image: dataUrl,
				name,
				width: document.body.scrollWidth,
				height: document.body.scrollHeight,
				regions: [
					{
						subId,
						x: Math.round(r.left + window.scrollX),
						y: Math.round(r.top + window.scrollY),
						w: Math.round(r.width),
						h: Math.round(r.height)
					}
				]
			})
		});
		if (res.status === 401 || res.status === 403) {
			writeToken = null;
			throw needsUnlock('Session expired — save again to sign in');
		}
		if (!res.ok) throw new Error(`Screenshot upload failed (HTTP ${res.status})`);
	}

	async function openFor(subId: number, el: Element): Promise<void> {
		try {
			const ctx = await resolveContext(subId);
			const live = Boolean(apiUrl && projectId);
			overlay.openEditor(ctx, {
				onSave: async (text) => {
					await save(ctx, text);
					overlay.closeEditor();
				},
				onUnlock: (email, password) => unlock(email, password),
				onCapture: live ? () => capture(subId, el, ctx.key.name) : undefined,
				onCancel: () => overlay.closeEditor()
			});
		} catch (e) {
			// Surface resolve failures via a fresh dialog so the user sees them.
			overlay.openEditor(
				{
					translation: { id: '', subId, text: '', state: 'error', languageId: '', version: 0 },
					key: { id: '', name: '(could not load)', description: '', namespaceId: '' },
					language: { tag: hijau.language, name: hijau.language, isRtl: false },
					sourceText: ''
				},
				{ onSave: () => overlay.closeEditor(), onCancel: () => overlay.closeEditor() }
			);
			overlay.setError((e as Error).message);
		}
	}

	function setActive(on: boolean): void {
		if (on === active) return;
		active = on;
		document.body.style.cursor = on ? 'crosshair' : '';
		if (!on) {
			hovered = null;
			overlay.highlight(null);
		}
	}

	const onKeyDown = (e: KeyboardEvent) => {
		if (e[modFlag]) setActive(true);
	};
	const onKeyUp = () => setActive(false);
	const onBlur = () => setActive(false);

	const onMouseMove = (e: MouseEvent) => {
		if (!e[modFlag]) {
			setActive(false);
			return;
		}
		setActive(true);
		if (overlay.isEditorOpen()) return;
		const hit = markedElementAt(e.target);
		hovered = hit?.el ?? null;
		overlay.highlight(hovered);
	};

	const onClick = (e: MouseEvent) => {
		if (!active && !e[modFlag]) return;
		const hit = markedElementAt(e.target);
		if (!hit) return;
		e.preventDefault();
		e.stopPropagation();
		setActive(false);
		void openFor(hit.subId, hit.el);
	};

	const onScroll = () => {
		if (active && hovered) overlay.highlight(hovered);
	};

	window.addEventListener('keydown', onKeyDown, true);
	window.addEventListener('keyup', onKeyUp, true);
	window.addEventListener('blur', onBlur);
	window.addEventListener('mousemove', onMouseMove, true);
	window.addEventListener('click', onClick, true);
	window.addEventListener('scroll', onScroll, true);

	return function disable() {
		window.removeEventListener('keydown', onKeyDown, true);
		window.removeEventListener('keyup', onKeyUp, true);
		window.removeEventListener('blur', onBlur);
		window.removeEventListener('mousemove', onMouseMove, true);
		window.removeEventListener('click', onClick, true);
		window.removeEventListener('scroll', onScroll, true);
		document.body.style.cursor = '';
		overlay.destroy();
		hijau.setMode(prevMode);
	};
}
