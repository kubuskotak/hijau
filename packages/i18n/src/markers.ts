// Zero-width "subliminal" marker codec — the linchpin of in-context editing.
//
// A translation's compact integer id (its DB sub_id) is encoded into a run of
// invisible zero-width characters and appended to the rendered string by the
// SDK. The in-context editor scans the DOM, decodes the markers back to the id,
// and maps the clicked node to the exact translation. The SDK (encode) and the
// editor (decode) must agree byte-for-byte, which is why this lives in one
// shared package.
//
// Encoding: a START sentinel, the id's binary digits (MSB first) followed by a
// 4-bit checksum (id mod 13), then an END sentinel — each bit rendered as one
// of two zero-width code points. The checksum lets the decoder reject stray
// zero-width characters from other libraries. Constants are defined by numeric
// code point so the source contains no invisible characters.

const ZERO = String.fromCharCode(0x200c); // ZERO WIDTH NON-JOINER -> bit 0
const ONE = String.fromCharCode(0x200d); //  ZERO WIDTH JOINER     -> bit 1
const START = String.fromCharCode(0x2062); // INVISIBLE TIMES       -> payload start
const END = String.fromCharCode(0x2063); //  INVISIBLE SEPARATOR   -> payload end

const CK_MOD = 13;
const CK_BITS = 4; // 0..12 fits in 4 bits

/** All marker code points, for stripping/detection. */
export const MARKER_CHARS = ZERO + ONE + START + END;

/** Encode a non-negative integer id into a zero-width marker string. */
export function encode(id: number): string {
	if (!Number.isInteger(id) || id < 0) {
		throw new Error('marker id must be a non-negative integer');
	}
	const bits = id.toString(2) + (id % CK_MOD).toString(2).padStart(CK_BITS, '0');
	let out = START;
	for (const b of bits) out += b === '1' ? ONE : ZERO;
	return out + END;
}

/** Decode the first marker found in a string back to its id, or null if none
 *  is present or the checksum fails. */
export function decode(s: string): number | null {
	const start = s.indexOf(START);
	if (start < 0) return null;
	const end = s.indexOf(END, start + 1);
	if (end < 0) return null;

	let bits = '';
	for (const ch of s.slice(start + 1, end)) {
		if (ch === ZERO) bits += '0';
		else if (ch === ONE) bits += '1';
		else return null; // foreign character inside the payload
	}
	if (bits.length <= CK_BITS) return null;

	const id = parseInt(bits.slice(0, -CK_BITS), 2);
	const ck = parseInt(bits.slice(-CK_BITS), 2);
	if (!Number.isFinite(id) || id % CK_MOD !== ck) return null;
	return id;
}

/** True if the string contains a valid marker payload. */
export function hasMarker(s: string): boolean {
	return decode(s) !== null;
}

/** Append a marker for `id` to `text` (no-op if already marked). */
export function wrap(text: string, id: number): string {
	if (text.includes(START)) return text;
	return text + encode(id);
}

/** Remove all marker characters/payloads, returning the visible text. */
export function unwrap(text: string): string {
	let out = '';
	for (const ch of text) {
		if (ch !== ZERO && ch !== ONE && ch !== START && ch !== END) out += ch;
	}
	return out;
}
