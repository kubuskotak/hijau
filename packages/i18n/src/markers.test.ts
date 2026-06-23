import { test, expect } from 'bun:test';
import { encode, decode, wrap, unwrap, hasMarker } from './markers';

const Z = String.fromCharCode(0x200c);
const O = String.fromCharCode(0x200d);
const S = String.fromCharCode(0x2062);
const E = String.fromCharCode(0x2063);

function build(idBits: string, ckBits: string): string {
	let s = S;
	for (const b of idBits + ckBits) s += b === '1' ? O : Z;
	return s + E;
}

test('roundtrip fixed ids', () => {
	for (const id of [0, 1, 2, 7, 12, 13, 42, 255, 1000, 65535, 123456789]) {
		expect(decode(encode(id))).toBe(id);
	}
});

test('roundtrip random ids', () => {
	for (let i = 0; i < 3000; i++) {
		const id = Math.floor(Math.random() * 1_000_000_000);
		expect(decode(encode(id))).toBe(id);
	}
});

test('marker decodes when embedded in surrounding text', () => {
	const s = 'before ' + encode(55) + ' after';
	expect(decode(s)).toBe(55);
});

test('wrap keeps visible text and embeds the id', () => {
	const w = wrap('Hello {name}', 4242);
	expect(unwrap(w)).toBe('Hello {name}');
	expect(decode(w)).toBe(4242);
	expect(w.length).toBeGreaterThan('Hello {name}'.length);
});

test('wrap is idempotent', () => {
	const once = wrap('Save', 5);
	expect(wrap(once, 99)).toBe(once);
});

test('all marker characters are zero-width sentinels', () => {
	for (const ch of encode(7)) expect(Z + O + S + E).toContain(ch);
});

test('decode returns null for plain strings', () => {
	expect(decode('just text {x}')).toBeNull();
	expect(hasMarker('nothing here')).toBe(false);
});

test('decode rejects a wrong checksum', () => {
	expect(decode(build('1000', '0000'))).toBeNull(); // id=8 but ck=0 (8 % 13 = 8)
	expect(decode(build('1000', '1000'))).toBe(8); // correct checksum
});

test('decode rejects foreign chars inside the payload', () => {
	const corrupt = S + Z + O + 'X' + E; // a visible char snuck into the payload
	expect(decode(corrupt)).toBeNull();
});
