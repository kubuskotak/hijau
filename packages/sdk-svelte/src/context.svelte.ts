// @hijau/svelte — Svelte 5 bindings for the in-context SDK.
//
// Put the client in context once near the root with setHijau(hijau), then read
// strings with translator() or the <T> component. Both track the SDK event bus
// via a rune, so an in-context edit re-renders live. In edit mode the text
// carries the invisible marker @hijau/incontext decodes.

import { getContext, setContext } from 'svelte';
import type { Hijau } from '@hijau/web';

const KEY = Symbol('hijau');

export function setHijau(client: Hijau): Hijau {
	setContext(KEY, client);
	return client;
}

export function getHijau(): Hijau {
	const client = getContext<Hijau | undefined>(KEY);
	if (!client) throw new Error('getHijau() must run in a component under setHijau()');
	return client;
}

/**
 * Returns a reactive `t(key, fallback?)`. Call it during component init; the
 * returned function reads a rune-tracked tick so templates that call it
 * re-render whenever the SDK emits (records loaded, mode flipped, edit saved).
 */
export function translator(): (key: string, fallback?: string) => string {
	const client = getHijau();
	let tick = $state(0);
	$effect(() => client.on(() => tick++));
	return (key: string, fallback?: string) => {
		void tick; // register the reactive dependency
		return client.t(key, fallback);
	};
}
