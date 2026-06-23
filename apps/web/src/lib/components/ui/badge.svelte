<script lang="ts" module>
	import { tv, type VariantProps } from 'tailwind-variants';

	export const badgeVariants = tv({
		base: 'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
		variants: {
			variant: {
				default: 'border-transparent bg-primary text-primary-foreground',
				secondary: 'border-transparent bg-secondary text-secondary-foreground',
				outline: 'text-foreground',
				// translation-state colours
				untranslated: 'border-transparent bg-muted text-muted-foreground',
				translated: 'border-transparent bg-blue-100 text-blue-700',
				reviewed: 'border-transparent bg-green-100 text-green-700',
				needs_work: 'border-transparent bg-amber-100 text-amber-800',
				outdated: 'border-transparent bg-orange-100 text-orange-800'
			}
		},
		defaultVariants: { variant: 'default' }
	});

	export type BadgeVariant = VariantProps<typeof badgeVariants>['variant'];
</script>

<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Snippet } from 'svelte';

	let {
		variant = 'default',
		class: className = undefined,
		children
	}: { variant?: BadgeVariant; class?: string; children?: Snippet } = $props();
</script>

<span class={cn(badgeVariants({ variant }), className)}>{@render children?.()}</span>
