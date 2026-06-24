import type { BadgeVariant } from '$lib/components/ui/badge/index';
import type { TranslationState } from '$lib/api';

/**
 * Maps a translation state to a presentation for the canonical shadcn-svelte
 * <Badge> (which only knows the generic variants). Domain colours live here so
 * the generated component stays pristine and can be re-pulled with
 * `bun x shadcn-svelte@latest add badge --overwrite` without losing anything.
 */
export interface StateBadge {
	variant: BadgeVariant;
	class: string;
	label: string;
}

const MAP: Record<TranslationState, StateBadge> = {
	untranslated: { variant: 'secondary', class: '', label: 'untranslated' },
	translated: { variant: 'default', class: 'text-sky-600 dark:text-sky-400', label: 'translated' },
	reviewed: { variant: 'default', class: 'text-emerald-600 dark:text-emerald-400', label: 'reviewed' },
	needs_work: { variant: 'destructive', class: '', label: 'needs work' },
	outdated: { variant: 'default', class: 'text-amber-600 dark:text-amber-500', label: 'outdated' }
};

export function stateBadge(state: string): StateBadge {
	return MAP[state as TranslationState] ?? { variant: 'secondary', class: '', label: state.replace('_', ' ') };
}
