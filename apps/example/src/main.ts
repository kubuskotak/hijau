// Demo wiring for the Hijau in-context SDK.
//
// Every element with a `data-hijau="<key>"` attribute is a translatable string.
// We render each one through `hijau.t(key)` and re-render whenever the SDK emits
// an event (so an in-context edit updates the page live). Hold Alt/Option and
// click any string to edit it.

import { createHijau, type TranslationRecord } from '@hijau/web';
import { enableInContext } from '@hijau/incontext';

// In a real app these come from `hijau pull --with-ids` at build time. The
// `subId` is the translation's DB id that the marker codec encodes.
const records: TranslationRecord[] = [
	{ subId: 23, key: 'nav.brand', text: 'Verdant Market' },
	{ subId: 26, key: 'nav.home', text: 'Home' },
	{ subId: 29, key: 'nav.shop', text: 'Shop' },
	{ subId: 32, key: 'nav.cart', text: 'Cart' },
	{ subId: 35, key: 'hero.title', text: 'Fresh groceries, delivered in 30 minutes' },
	{ subId: 38, key: 'hero.subtitle', text: 'Locally sourced, always in season.' },
	{ subId: 41, key: 'hero.cta', text: 'Start shopping' },
	{ subId: 44, key: 'feature.delivery.title', text: 'Free delivery' },
	{ subId: 47, key: 'feature.delivery.body', text: 'On every order over $35.' },
	{ subId: 50, key: 'feature.window.title', text: 'Your window' },
	{ subId: 53, key: 'feature.window.body', text: 'Pick a delivery time that suits you.' },
	{ subId: 56, key: 'feature.guarantee.title', text: 'Happiness guarantee' },
	{ subId: 59, key: 'feature.guarantee.body', text: "Not happy? We'll make it right." },
	{ subId: 62, key: 'cart.empty', text: 'Your cart is empty.' }
];

// Flip to a live Hijau instance by filling these in. `token` is a read-only
// editor token (POST /projects/:id/editor/token). With the vite proxy,
// '/api/v1' reaches :8080. Saving then prompts for sign-in (unlock) so edits
// persist to the server, attributed to the real person.
// Seeded demo project on the local instance. Sign in to save with
// demo@hijau.dev / hijaudemo123.
const LIVE: { apiUrl?: string; projectId?: string; token?: string } = {
	apiUrl: '/api/v1',
	projectId: '01KVV1PPP4MV5MS2P6HE01K7CM',
	token: 'hj_edit_9RO8F0LMO_9sMVjEoIHyscmHMf7JkSga'
};

const hijau = createHijau({
	language: 'en',
	records,
	mode: 'production',
	apiUrl: LIVE.apiUrl,
	projectId: LIVE.projectId
});

function render(): void {
	for (const el of document.querySelectorAll<HTMLElement>('[data-hijau]')) {
		const key = el.dataset.hijau;
		if (key) el.textContent = hijau.t(key);
	}
}

// Re-render on any SDK event (records loaded, mode flip, translation updated).
hijau.on(render);
render();

const toggle = document.getElementById('toggle') as HTMLButtonElement;
const status = document.getElementById('status') as HTMLSpanElement;
let disable: (() => void) | null = null;

toggle.addEventListener('click', () => {
	if (disable) {
		disable();
		disable = null;
		toggle.textContent = 'Enable in-context editing';
		toggle.classList.remove('on');
		status.textContent = '';
	} else {
		disable = enableInContext(hijau, {
			apiUrl: LIVE.apiUrl,
			projectId: LIVE.projectId,
			token: LIVE.token,
			onSave: (ctx, text) => {
				status.textContent = `Saved “${ctx.key.name}” → ${text}`;
			}
		});
		toggle.textContent = 'Disable in-context editing';
		toggle.classList.add('on');
		status.textContent = 'Hold Alt/Option and click a string…';
	}
});
