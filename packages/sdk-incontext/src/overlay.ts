// The overlay editor UI, rendered inside a Shadow DOM so the host app's styles
// (and its CSS resets) can never leak in or out. It draws three things:
//   1. a highlight box that tracks the translatable element under the cursor;
//   2. a modal dialog to edit one string (source for reference + a textarea);
//   3. an inline unlock panel (email + password) shown when a save needs the
//      user to re-authenticate.
// It owns no business logic — the controller in index.ts drives it.

import { unwrap } from '@hijau/i18n';
import type { EditContext } from './types';

const STYLE = `
:host { all: initial; }
* { box-sizing: border-box; font-family: ui-sans-serif, system-ui, sans-serif; }
.hl {
	position: fixed; pointer-events: none; z-index: 2147483646;
	border: 2px solid #16a34a; border-radius: 4px;
	background: rgba(22,163,74,0.08); transition: all 60ms ease; display: none;
}
.backdrop {
	position: fixed; inset: 0; z-index: 2147483647;
	background: rgba(0,0,0,0.35); display: none;
	align-items: center; justify-content: center; padding: 16px;
}
.dialog {
	width: 100%; max-width: 32rem; background: #fff; color: #18181b;
	border-radius: 12px; box-shadow: 0 20px 60px rgba(0,0,0,0.3);
	overflow: hidden; font-size: 14px;
}
.head { padding: 14px 16px; border-bottom: 1px solid #e4e4e7; }
.title { font-weight: 600; word-break: break-all; }
.sub { color: #71717a; font-size: 12px; margin-top: 2px; }
.body { padding: 16px; display: flex; flex-direction: column; gap: 12px; }
.label { font-size: 11px; font-weight: 600; text-transform: uppercase;
	letter-spacing: 0.06em; color: #71717a; }
.source { background: #f4f4f5; border-radius: 8px; padding: 8px 10px; white-space: pre-wrap; }
textarea, input {
	width: 100%; padding: 10px; border: 1px solid #d4d4d8; border-radius: 8px;
	font: inherit; color: inherit; background: #fff;
}
textarea { min-height: 84px; resize: vertical; }
textarea:focus, input:focus { outline: 2px solid #16a34a; border-color: #16a34a; }
.unlock { display: none; flex-direction: column; gap: 8px;
	border-top: 1px dashed #e4e4e7; padding-top: 12px; }
.unlock.show { display: flex; }
.unlock .hint { font-size: 12px; color: #71717a; }
.row { display: flex; flex-direction: column; gap: 4px; }
.foot { padding: 12px 16px; border-top: 1px solid #e4e4e7;
	display: flex; gap: 8px; justify-content: flex-end; align-items: center; }
.err { color: #dc2626; font-size: 12px; margin-right: auto; }
button { font: inherit; border-radius: 8px; padding: 7px 14px; cursor: pointer;
	border: 1px solid transparent; }
.ghost { background: transparent; border-color: #d4d4d8; color: #18181b; }
.primary { background: #16a34a; color: #fff; }
button:disabled { opacity: 0.6; cursor: default; }
.badge { display: inline-block; font-size: 11px; padding: 1px 6px; border-radius: 999px;
	background: #e4e4e7; color: #3f3f46; margin-left: 6px; vertical-align: middle; }
`;

export interface OverlayHandlers {
	onSave: (text: string) => void | Promise<void>;
	onUnlock?: (email: string, password: string) => void | Promise<void>;
	onCancel: () => void;
}

/** A save handler can throw an error carrying this flag to ask the overlay to
 *  reveal the unlock panel instead of just showing the message. */
export interface UnlockableError extends Error {
	needsUnlock?: boolean;
}

export class Overlay {
	private host: HTMLElement;
	private root: ShadowRoot;
	private hl: HTMLDivElement;
	private backdrop: HTMLDivElement;
	private titleEl!: HTMLDivElement;
	private subEl!: HTMLDivElement;
	private sourceEl!: HTMLDivElement;
	private textarea!: HTMLTextAreaElement;
	private unlockEl!: HTMLDivElement;
	private emailEl!: HTMLInputElement;
	private passwordEl!: HTMLInputElement;
	private errEl!: HTMLDivElement;
	private saveBtn!: HTMLButtonElement;
	private handlers: OverlayHandlers | null = null;
	private mode: 'edit' | 'unlock' = 'edit';

	constructor() {
		this.host = document.createElement('div');
		this.host.setAttribute('data-hijau-overlay', '');
		this.root = this.host.attachShadow({ mode: 'open' });

		const style = document.createElement('style');
		style.textContent = STYLE;
		this.root.appendChild(style);

		this.hl = document.createElement('div');
		this.hl.className = 'hl';
		this.root.appendChild(this.hl);

		this.backdrop = document.createElement('div');
		this.backdrop.className = 'backdrop';
		this.backdrop.innerHTML = `
			<div class="dialog" role="dialog" aria-modal="true">
				<div class="head">
					<div class="title"></div>
					<div class="sub"></div>
				</div>
				<div class="body">
					<div>
						<div class="label">Source</div>
						<div class="source"></div>
					</div>
					<div>
						<div class="label">Translation</div>
						<textarea></textarea>
					</div>
					<div class="unlock">
						<div class="hint">Sign in to save your changes. Edits are recorded under your name.</div>
						<div class="row"><div class="label">Email</div><input type="email" autocomplete="username" /></div>
						<div class="row"><div class="label">Password</div><input type="password" autocomplete="current-password" /></div>
					</div>
				</div>
				<div class="foot">
					<span class="err"></span>
					<button class="ghost" data-act="cancel">Cancel</button>
					<button class="primary" data-act="save">Save</button>
				</div>
			</div>`;
		this.root.appendChild(this.backdrop);

		this.titleEl = this.q('.title');
		this.subEl = this.q('.sub');
		this.sourceEl = this.q('.source');
		this.textarea = this.q('textarea');
		this.unlockEl = this.q('.unlock');
		const inputs = this.root.querySelectorAll('input');
		this.emailEl = inputs[0] as HTMLInputElement;
		this.passwordEl = inputs[1] as HTMLInputElement;
		this.errEl = this.q('.err');
		this.saveBtn = this.q('button[data-act="save"]');

		this.backdrop.addEventListener('click', (e) => {
			if (e.target === this.backdrop) this.handlers?.onCancel();
		});
		this.q('button[data-act="cancel"]').addEventListener('click', () => this.handlers?.onCancel());
		this.saveBtn.addEventListener('click', () => void this.onPrimary());
		const submitOnCtrlEnter = (e: KeyboardEvent) => {
			if (e.key === 'Escape') this.handlers?.onCancel();
			if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) void this.onPrimary();
		};
		this.textarea.addEventListener('keydown', submitOnCtrlEnter);
		this.passwordEl.addEventListener('keydown', (e) => {
			if (e.key === 'Enter') void this.onPrimary();
		});
	}

	private q<T extends Element>(sel: string): T {
		return this.root.querySelector(sel) as T;
	}

	mount(): void {
		if (!this.host.isConnected) document.body.appendChild(this.host);
	}

	destroy(): void {
		this.host.remove();
	}

	/** Position the hover highlight over an element (or hide it if null). */
	highlight(el: Element | null): void {
		if (!el) {
			this.hl.style.display = 'none';
			return;
		}
		const r = el.getBoundingClientRect();
		this.hl.style.display = 'block';
		this.hl.style.left = `${r.left - 2}px`;
		this.hl.style.top = `${r.top - 2}px`;
		this.hl.style.width = `${r.width + 4}px`;
		this.hl.style.height = `${r.height + 4}px`;
	}

	openEditor(ctx: EditContext, handlers: OverlayHandlers): void {
		this.handlers = handlers;
		this.titleEl.textContent = ctx.key.name;
		this.subEl.innerHTML = `${ctx.language.name} (${ctx.language.tag})<span class="badge">${ctx.translation.state}</span>`;
		this.sourceEl.textContent = ctx.sourceText || '—';
		this.textarea.value = unwrap(ctx.translation.text);
		this.textarea.dir = ctx.language.isRtl ? 'rtl' : 'ltr';
		this.setError('');
		this.setBusy(false);
		this.setMode('edit');
		this.highlight(null);
		this.backdrop.style.display = 'flex';
		this.textarea.focus();
		this.textarea.setSelectionRange(this.textarea.value.length, this.textarea.value.length);
	}

	closeEditor(): void {
		this.backdrop.style.display = 'none';
		this.passwordEl.value = '';
		this.handlers = null;
	}

	isEditorOpen(): boolean {
		return this.backdrop.style.display === 'flex';
	}

	setError(msg: string): void {
		this.errEl.textContent = msg;
	}

	private setMode(mode: 'edit' | 'unlock'): void {
		this.mode = mode;
		this.unlockEl.classList.toggle('show', mode === 'unlock');
		this.saveBtn.textContent = mode === 'unlock' ? 'Unlock & save' : 'Save';
	}

	private setBusy(busy: boolean): void {
		this.saveBtn.disabled = busy;
		if (busy) this.saveBtn.textContent = this.mode === 'unlock' ? 'Unlocking…' : 'Saving…';
		else this.saveBtn.textContent = this.mode === 'unlock' ? 'Unlock & save' : 'Save';
	}

	private async onPrimary(): Promise<void> {
		if (!this.handlers) return;
		this.setBusy(true);
		this.setError('');
		try {
			if (this.mode === 'unlock' && this.handlers.onUnlock) {
				await this.handlers.onUnlock(this.emailEl.value.trim(), this.passwordEl.value);
				this.passwordEl.value = '';
			}
			await this.handlers.onSave(this.textarea.value);
		} catch (e) {
			const err = e as UnlockableError;
			if (err.needsUnlock && this.handlers.onUnlock) {
				this.setMode('unlock');
				this.emailEl.focus();
			}
			this.setError(err.message || 'Save failed');
			this.setBusy(false);
		}
	}
}
