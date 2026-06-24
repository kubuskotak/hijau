<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Language, type GlossaryTerm } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Badge } from '$lib/components/ui/badge/index';
	import { Textarea } from '$lib/components/ui/textarea/index';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const languages = $derived(data.languages as Language[]);

	let terms = $state<GlossaryTerm[]>([]);
	let loading = $state(true);
	let err = $state('');

	let newTerm = $state('');
	let newDescription = $state('');
	let newCaseSensitive = $state(false);
	let newDoNotTranslate = $state(false);
	let busy = $state('');

	// Per-term, per-language draft translation values keyed `${termId}:${languageId}`.
	let drafts = $state<Record<string, string>>({});

	function draftKey(termId: string, languageId: string): string {
		return `${termId}:${languageId}`;
	}

	function seedDrafts(list: GlossaryTerm[]) {
		const next: Record<string, string> = {};
		for (const t of list) {
			for (const l of languages) {
				next[draftKey(t.id, l.id)] = t.translations?.[l.id] ?? '';
			}
		}
		drafts = next;
	}

	async function load() {
		loading = true;
		err = '';
		try {
			const list = await api.listGlossary(pid);
			terms = list;
			seedDrafts(list);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function addTerm(e: SubmitEvent) {
		e.preventDefault();
		if (!newTerm.trim()) return;
		busy = 'add';
		err = '';
		try {
			const created = await api.createGlossaryTerm(pid, {
				term: newTerm.trim(),
				description: newDescription.trim() || undefined,
				caseSensitive: newCaseSensitive,
				doNotTranslate: newDoNotTranslate
			});
			terms = [...terms, created];
			for (const l of languages) {
				drafts[draftKey(created.id, l.id)] = created.translations?.[l.id] ?? '';
			}
			newTerm = '';
			newDescription = '';
			newCaseSensitive = false;
			newDoNotTranslate = false;
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}

	async function removeTerm(termId: string) {
		busy = `del:${termId}`;
		err = '';
		try {
			await api.deleteGlossaryTerm(pid, termId);
			terms = terms.filter((t) => t.id !== termId);
			for (const l of languages) {
				delete drafts[draftKey(termId, l.id)];
			}
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}

	async function saveTranslation(term: GlossaryTerm, language: Language) {
		const key = draftKey(term.id, language.id);
		busy = `tr:${key}`;
		err = '';
		try {
			const value = drafts[key] ?? '';
			await api.setGlossaryTranslation(pid, term.id, language.tag, value);
			// Reflect the saved value in local state so it survives a refetch-free update.
			term.translations = { ...term.translations, [language.id]: value };
			terms = terms.map((t) => (t.id === term.id ? term : t));
		} catch (e) {
			err = (e as Error).message;
		} finally {
			busy = '';
		}
	}
</script>

<div class="grid gap-8 lg:grid-cols-3">
	<div class="lg:col-span-2">
		<h2 class="text-lg font-medium">Glossary</h2>
		<p class="text-sm text-muted-foreground">
			Terms guide translators and machine translation toward consistent, approved wording.
		</p>

		{#if loading}
			<p class="mt-4 text-sm text-muted-foreground">Loading…</p>
		{:else if terms.length === 0}
			<p class="mt-4 text-sm text-muted-foreground">No glossary terms yet.</p>
		{:else}
			<ul class="mt-4 space-y-4">
				{#each terms as term (term.id)}
					<li class="rounded-xl border bg-card p-4 shadow-sm">
						<div class="flex items-start justify-between gap-3">
							<div>
								<div class="flex flex-wrap items-center gap-2">
									<span class="font-medium">{term.term}</span>
									{#if term.doNotTranslate}
										<Badge variant="secondary">Do not translate</Badge>
									{/if}
									{#if term.caseSensitive}
										<Badge variant="outline">Case-sensitive</Badge>
									{/if}
								</div>
								{#if term.description}
									<p class="mt-1 text-sm text-muted-foreground">{term.description}</p>
								{/if}
							</div>
							<Button
								variant="outline"
								size="sm"
								onclick={() => removeTerm(term.id)}
								disabled={busy === `del:${term.id}`}
							>
								Delete
							</Button>
						</div>

						{#if languages.length > 0}
							<div class="mt-3 space-y-3 border-t pt-3">
								{#each languages as l (l.id)}
									{@const key = draftKey(term.id, l.id)}
									<div class="space-y-1.5">
										<Label for={`tr-${key}`}>{l.name} ({l.tag})</Label>
										<div class="flex items-center gap-2">
											<Input
												id={`tr-${key}`}
												bind:value={drafts[key]}
												placeholder="Approved term…"
												dir={l.isRtl ? 'rtl' : undefined}
											/>
											<Button
												variant="outline"
												size="sm"
												onclick={() => saveTranslation(term, l)}
												disabled={busy === `tr:${key}`}
											>
												{busy === `tr:${key}` ? 'Saving…' : 'Save'}
											</Button>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

		{#if err}<p class="mt-3 text-sm text-destructive">{err}</p>{/if}
	</div>

	<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={addTerm}>
		<h3 class="font-medium">Add a term</h3>
		<div class="mt-3 space-y-3">
			<div class="space-y-1.5">
				<Label for="g-term">Term</Label>
				<Input id="g-term" bind:value={newTerm} placeholder="Dashboard" required />
			</div>
			<div class="space-y-1.5">
				<Label for="g-description">Description</Label>
				<Textarea
					id="g-description"
					bind:value={newDescription}
					placeholder="Notes for translators…"
				/>
			</div>
			<label class="flex items-center gap-2 text-sm">
				<input
					type="checkbox"
					bind:checked={newCaseSensitive}
					class="h-4 w-4 rounded border-input"
				/>
				Case-sensitive
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input
					type="checkbox"
					bind:checked={newDoNotTranslate}
					class="h-4 w-4 rounded border-input"
				/>
				Do not translate
			</label>
			<Button type="submit" disabled={busy === 'add'}>
				{busy === 'add' ? 'Adding…' : 'Add term'}
			</Button>
		</div>
	</form>
</div>
