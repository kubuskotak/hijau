import { describe, expect, test } from 'bun:test';
import { decode, unwrap } from '@hijau/i18n';
import { createHijau, type HijauEvent, type TranslationRecord } from './index';

const records: TranslationRecord[] = [
	{ subId: 1, key: 'cart.checkout', text: 'Checkout' },
	{ subId: 42, key: 'cart.empty', text: 'Your cart is empty' }
];

describe('createHijau', () => {
	test('production mode renders plain strings (no markers)', () => {
		const h = createHijau({ language: 'en', records });
		const out = h.t('cart.checkout');
		expect(out).toBe('Checkout');
		expect(decode(out)).toBeNull();
	});

	test('edit mode appends a marker that decodes to the record subId', () => {
		const h = createHijau({ language: 'en', records, mode: 'edit' });
		const out = h.t('cart.empty');
		expect(unwrap(out)).toBe('Your cart is empty');
		expect(decode(out)).toBe(42);
	});

	test('unknown key falls back to fallback then to the key, never marked', () => {
		const h = createHijau({ language: 'en', records, mode: 'edit' });
		expect(h.t('missing', 'Default')).toBe('Default');
		expect(h.t('missing')).toBe('missing');
		expect(decode(h.t('missing', 'Default'))).toBeNull();
	});

	test('lookup by key and by subId', () => {
		const h = createHijau({ language: 'en', records });
		expect(h.getRecord('cart.checkout')?.subId).toBe(1);
		expect(h.getRecordBySubId(42)?.key).toBe('cart.empty');
		expect(h.getRecord('nope')).toBeUndefined();
	});

	test('applyUpdate mutates the store and emits translation.updated', () => {
		const h = createHijau({ language: 'en', records: structuredClone(records), mode: 'edit' });
		const events: HijauEvent[] = [];
		h.on((e) => events.push(e));

		h.applyUpdate(1, 'Pay now');
		expect(h.getRecordBySubId(1)?.text).toBe('Pay now');
		expect(unwrap(h.t('cart.checkout'))).toBe('Pay now');
		expect(events).toEqual([{ type: 'translation.updated', subId: 1, key: 'cart.checkout', text: 'Pay now' }]);
	});

	test('applyUpdate on an unknown subId is a no-op', () => {
		const h = createHijau({ language: 'en', records: structuredClone(records) });
		const events: HijauEvent[] = [];
		h.on((e) => events.push(e));
		h.applyUpdate(999, 'x');
		expect(events).toHaveLength(0);
	});

	test('setMode flips rendering and emits once (deduped)', () => {
		const h = createHijau({ language: 'en', records });
		const events: HijauEvent[] = [];
		h.on((e) => events.push(e));

		expect(decode(h.t('cart.checkout'))).toBeNull();
		h.setMode('edit');
		expect(decode(h.t('cart.checkout'))).toBe(1);
		h.setMode('edit'); // no change → no event
		expect(events).toEqual([{ type: 'mode.changed', mode: 'edit' }]);
	});

	test('setRecords re-indexes and emits records.loaded', () => {
		const h = createHijau({ language: 'en' });
		const events: HijauEvent[] = [];
		h.on((e) => events.push(e));
		h.setRecords(records);
		expect(h.getRecord('cart.empty')?.subId).toBe(42);
		expect(events).toEqual([{ type: 'records.loaded', count: 2 }]);
	});

	test('on() returns an unsubscribe that stops delivery', () => {
		const h = createHijau({ language: 'en', records: structuredClone(records) });
		const events: HijauEvent[] = [];
		const off = h.on((e) => events.push(e));
		off();
		h.applyUpdate(1, 'x');
		expect(events).toHaveLength(0);
	});
});
