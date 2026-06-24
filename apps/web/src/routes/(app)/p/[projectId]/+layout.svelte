<script lang="ts">
	import { page } from '$app/stores';
	import { cn } from '$lib/utils';

	let { data, children } = $props();
	const pid = $derived(data.project.id);

	const tabs = $derived([
		{ href: `/p/${pid}`, label: 'Editor' },
		{ href: `/p/${pid}/languages`, label: 'Languages' },
		{ href: `/p/${pid}/glossary`, label: 'Glossary' },
		{ href: `/p/${pid}/io`, label: 'Import / Export' },
		{ href: `/p/${pid}/webhooks`, label: 'Webhooks' }
	]);
</script>

<div class="space-y-6">
	<div>
		<a href="/" class="text-xs text-muted-foreground hover:underline">← Projects</a>
		<h1 class="mt-1 text-2xl font-semibold tracking-tight">{data.project.name}</h1>
		{#if data.project.description}
			<p class="text-sm text-muted-foreground">{data.project.description}</p>
		{/if}
	</div>

	<nav class="flex gap-1 border-b">
		{#each tabs as t (t.href)}
			<a
				href={t.href}
				class={cn(
					'-mb-px border-b-2 px-3 py-2 text-sm transition-colors',
					$page.url.pathname === t.href
						? 'border-foreground font-medium text-foreground'
						: 'border-transparent text-muted-foreground hover:text-foreground'
				)}
			>
				{t.label}
			</a>
		{/each}
	</nav>

	{@render children()}
</div>
