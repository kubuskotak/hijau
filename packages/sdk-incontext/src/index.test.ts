import { describe, expect, test } from 'bun:test';
import { encode } from '@hijau/i18n';
import { createHijau } from '@hijau/web';
import { enableInContext, markedElementAt } from './index';

// Minimal element stub: just what markedElementAt touches.
function el(text: string, parent: unknown = null) {
	return { textContent: text, parentElement: parent };
}

describe('markedElementAt', () => {
	test('finds the sub_id on a leaf element carrying a marker', () => {
		const leaf = el('Bonjour' + encode(42));
		const hit = markedElementAt(leaf as unknown as EventTarget);
		expect(hit?.subId).toBe(42);
		expect(hit?.el).toBe(leaf as never);
	});

	test('climbs to a marked ancestor when the target itself is unmarked', () => {
		const parent = el('Hello' + encode(7));
		const child = el('plain child', parent);
		const hit = markedElementAt(child as unknown as EventTarget);
		expect(hit?.subId).toBe(7);
		expect(hit?.el).toBe(parent as never);
	});

	test('returns null when nothing in the chain is marked', () => {
		const child = el('plain', el('also plain'));
		expect(markedElementAt(child as unknown as EventTarget)).toBeNull();
	});

	test('returns null for null / non-node targets', () => {
		expect(markedElementAt(null)).toBeNull();
		expect(markedElementAt({} as EventTarget)).toBeNull();
	});

	test('stops climbing after the depth cap', () => {
		let node: unknown = el('marked' + encode(1));
		for (let i = 0; i < 20; i++) node = el('plain', node); // bury the marker deep
		expect(markedElementAt(node as EventTarget)).toBeNull();
	});
});

describe('enableInContext (no DOM)', () => {
	test('is a safe no-op returning a disable function when document is absent', () => {
		const h = createHijau({ language: 'en', records: [{ subId: 1, key: 'k', text: 'v' }] });
		const disable = enableInContext(h);
		expect(typeof disable).toBe('function');
		expect(() => disable()).not.toThrow();
		// mode must be untouched in a non-DOM environment
		expect(h.mode).toBe('production');
	});
});
