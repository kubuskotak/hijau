<script lang="ts">
	import { cn } from '$lib/utils';
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	let {
		class: className = undefined,
		value = $bindable(),
		...rest
	}: HTMLTextareaAttributes & { class?: string } = $props();
</script>

<textarea
	bind:value
	class={cn(
		'flex min-h-9 w-full resize-y rounded-md border border-input bg-transparent px-3 py-1.5 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50',
		className
	)}
	{...rest}
></textarea>
