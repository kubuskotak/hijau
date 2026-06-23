<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, type Language } from '$lib/api';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import Label from '$lib/components/ui/label.svelte';
	import Badge from '$lib/components/ui/badge.svelte';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const baseId = $derived(data.project.baseLanguageId);
	const languages = $derived(data.languages as Language[]);

	let tag = $state('');
	let name = $state('');
	let isRtl = $state(false);
	let plurals = $state('one, other');
	let busy = $state('');
	let err = $state('');

	async function addLanguage(e: SubmitEvent) {
		e.preventDefault();
		if (!tag.trim() || !name.trim()) return;
		busy = 'add';
		err = '';
		try {
			await api.createLanguage(pid, {
				tag: tag.trim(),
				name: name.trim(),
				isRtl,
				pluralForms: plurals
					.split(',')
					.map((s) => s.trim())
					.filter(Boolean)
			});
			tag = '';
			name = '';
			isRtl = false;
			await invalidateAll();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}

	async function setBase(id: string) {
		busy = id;
		err = '';
		try {
			await api.setBaseLanguage(pid, id);
			await invalidateAll();
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}
</script>

<div class="grid gap-8 lg:grid-cols-2">
	<div>
		<h2 class="text-lg font-medium">Languages</h2>
		<p class="text-sm text-muted-foreground">
			The base language holds the source strings; editing it marks other languages outdated.
		</p>
		{#if languages.length === 0}
			<p class="mt-4 text-sm text-muted-foreground">No languages yet.</p>
		{:else}
			<ul class="mt-4 divide-y rounded-xl border">
				{#each languages as l (l.id)}
					<li class="flex items-center justify-between px-4 py-3">
						<div>
							<span class="font-medium">{l.name}</span>
							<span class="text-sm text-muted-foreground">({l.tag})</span>
							{#if l.isRtl}<span class="ml-2 text-xs text-muted-foreground">RTL</span>{/if}
						</div>
						{#if l.id === baseId}
							<Badge variant="secondary">Base</Badge>
						{:else}
							<Button variant="outline" size="sm" onclick={() => setBase(l.id)} disabled={busy === l.id}>
								Set as base
							</Button>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
		{#if err}<p class="mt-3 text-sm text-destructive">{err}</p>{/if}
	</div>

	<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={addLanguage}>
		<h3 class="font-medium">Add a language</h3>
		<div class="mt-3 space-y-3">
			<div class="grid grid-cols-2 gap-3">
				<div class="space-y-1.5">
					<Label for="l-tag">BCP-47 tag</Label>
					<Input id="l-tag" bind:value={tag} placeholder="pt-BR" required />
				</div>
				<div class="space-y-1.5">
					<Label for="l-name">Name</Label>
					<Input id="l-name" bind:value={name} placeholder="Portuguese (Brazil)" required />
				</div>
			</div>
			<div class="space-y-1.5">
				<Label for="l-plurals">Plural forms (CLDR)</Label>
				<Input id="l-plurals" bind:value={plurals} placeholder="one, other" />
			</div>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={isRtl} class="h-4 w-4 rounded border-input" />
				Right-to-left script
			</label>
			<Button type="submit" disabled={busy === 'add'}>{busy === 'add' ? 'Adding…' : 'Add language'}</Button>
		</div>
	</form>
</div>
