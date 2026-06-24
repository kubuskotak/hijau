// @hijau/react — React bindings for the in-context SDK.
//
// Provide the client once at the root with <HijauProvider client={hijau}>,
// then render strings with useTranslate() or <T translationKey="…" />. Both
// subscribe to the SDK event bus, so an in-context edit re-renders the tree
// live. In edit mode the returned text carries the invisible marker that
// @hijau/incontext decodes — no extra wiring at the call site.
//
// Written with createElement (no JSX) so the package needs no JSX build config.

import { createContext, createElement, useContext, useEffect, useReducer, type ReactNode } from 'react';
import type { Hijau } from '@hijau/web';

const HijauContext = createContext<Hijau | null>(null);

export function HijauProvider(props: { client: Hijau; children?: ReactNode }): ReactNode {
	return createElement(HijauContext.Provider, { value: props.client }, props.children);
}

export function useHijau(): Hijau {
	const client = useContext(HijauContext);
	if (!client) throw new Error('useHijau must be used within a <HijauProvider>');
	return client;
}

/** Re-render the calling component whenever the SDK emits an event. */
function useHijauRerender(client: Hijau): void {
	const [, force] = useReducer((c: number) => c + 1, 0);
	useEffect(() => client.on(() => force()), [client]);
}

/** Returns a `t(key, fallback?)` bound to the provider's client. The component
 *  re-renders on edits, so each render calls `t` and gets fresh text. */
export function useTranslate(): (key: string, fallback?: string) => string {
	const client = useHijau();
	useHijauRerender(client);
	return (key, fallback) => client.t(key, fallback);
}

/** Renders one translation as text. `translationKey` (not `key`, which React
 *  reserves) names the string. */
export function T(props: { translationKey: string; fallback?: string }): ReactNode {
	const t = useTranslate();
	return t(props.translationKey, props.fallback);
}
