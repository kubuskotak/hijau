<script lang="ts">
	import { api, type EditorRow, type Language, type HistoryEntry, type Comment } from '$lib/api';
	import Button from '$lib/components/ui/button.svelte';
	import Textarea from '$lib/components/ui/textarea.svelte';
	import Badge from '$lib/components/ui/badge.svelte';
	import type { BadgeVariant } from '$lib/components/ui/badge.svelte';

	let {
		pid,
		keyRow,
		lang,
		baseText,
		onclose
	}: {
		pid: string;
		keyRow: EditorRow;
		lang: Language;
		baseText: string;
		onclose: () => void;
	} = $props();

	let tab = $state<'history' | 'comments'>('history');
	let history = $state<HistoryEntry[]>([]);
	let comments = $state<Comment[]>([]);
	let loading = $state(true);
	let err = $state('');
	let draft = $state('');
	let posting = $state(false);

	async function loadFor(kid: string, langTag: string) {
		loading = true;
		err = '';
		try {
			[history, comments] = await Promise.all([
				api.translationHistory(pid, kid, langTag),
				api.listComments(pid, kid, langTag)
			]);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	// Reload whenever the focused cell changes.
	$effect(() => {
		void loadFor(keyRow.id, lang.tag);
	});

	async function postComment(e: SubmitEvent) {
		e.preventDefault();
		const body = draft.trim();
		if (!body) return;
		posting = true;
		try {
			const c = await api.addComment(pid, keyRow.id, lang.tag, body);
			comments = [...comments, c];
			draft = '';
		} catch (e) {
			err = (e as Error).message;
		} finally {
			posting = false;
		}
	}

	async function toggleResolve(c: Comment) {
		try {
			await api.resolveComment(c.id, !c.resolved);
			comments = comments.map((x) => (x.id === c.id ? { ...x, resolved: !x.resolved } : x));
		} catch (e) {
			err = (e as Error).message;
		}
	}

	function fmt(ts: string): string {
		return new Date(ts).toLocaleString();
	}
</script>

<!-- backdrop -->
<div
	class="fixed inset-0 z-40 bg-black/20"
	role="button"
	tabindex="-1"
	onclick={onclose}
	onkeydown={(e) => e.key === 'Escape' && onclose()}
></div>

<aside
	class="fixed inset-y-0 right-0 z-50 flex w-[26rem] max-w-[90vw] flex-col border-l bg-card shadow-xl"
>
	<header class="flex items-start justify-between gap-2 border-b p-4">
		<div class="min-w-0">
			<div class="truncate font-medium">{keyRow.name}</div>
			<div class="text-xs text-muted-foreground">
				{lang.name} ({lang.tag})
			</div>
		</div>
		<Button variant="ghost" size="icon" class="h-7 w-7" onclick={onclose} aria-label="Close">✕</Button>
	</header>

	<div class="space-y-3 border-b p-4 text-sm">
		<div>
			<div class="text-xs font-medium text-muted-foreground">Source</div>
			<div class="mt-0.5 rounded-md bg-muted/50 px-2 py-1">{baseText || '—'}</div>
		</div>
		{#if keyRow.description}
			<div>
				<div class="text-xs font-medium text-muted-foreground">Description</div>
				<div class="mt-0.5">{keyRow.description}</div>
			</div>
		{/if}
		{#if keyRow.namespaceId}
			<div class="text-xs text-muted-foreground">Namespace: {keyRow.namespaceId}</div>
		{/if}
	</div>

	<nav class="flex gap-1 border-b px-2">
		{#each ['history', 'comments'] as const as t (t)}
			<button
				class={'border-b-2 px-3 py-2 text-sm capitalize ' +
					(tab === t
						? 'border-foreground font-medium'
						: 'border-transparent text-muted-foreground hover:text-foreground')}
				onclick={() => (tab = t)}
			>
				{t}
				{#if t === 'comments' && comments.length}({comments.length}){/if}
			</button>
		{/each}
	</nav>

	<div class="flex-1 overflow-y-auto p-4">
		{#if err}<p class="text-sm text-destructive">{err}</p>{/if}
		{#if loading}
			<p class="text-sm text-muted-foreground">Loading…</p>
		{:else if tab === 'history'}
			{#if history.length === 0}
				<p class="text-sm text-muted-foreground">No history yet.</p>
			{:else}
				<ol class="space-y-3">
					{#each history as h (h.id)}
						<li class="border-l-2 pl-3 text-sm">
							<div class="flex items-center gap-2">
								{#if h.newState}<Badge variant={h.newState as BadgeVariant}>
										{h.newState.replace('_', ' ')}
									</Badge>{/if}
								<span class="text-xs text-muted-foreground">{fmt(h.createdAt)}</span>
							</div>
							<div class="mt-1 text-xs text-muted-foreground">
								{h.authorEmail || h.authorKind}
							</div>
							{#if h.newText !== h.oldText}
								<div class="mt-1 whitespace-pre-wrap">
									{#if h.oldText}<span class="text-muted-foreground line-through">{h.oldText}</span>
										→
									{/if}<span>{h.newText || '—'}</span>
								</div>
							{/if}
						</li>
					{/each}
				</ol>
			{/if}
		{:else}
			<div class="space-y-3">
				{#if comments.length === 0}
					<p class="text-sm text-muted-foreground">No comments yet.</p>
				{:else}
					<ul class="space-y-3">
						{#each comments as c (c.id)}
							<li class={'rounded-md border p-2 text-sm ' + (c.parentId ? 'ml-4' : '')}>
								<div class="flex items-center justify-between gap-2">
									<span class="text-xs font-medium">{c.authorName || c.authorEmail}</span>
									<span class="text-xs text-muted-foreground">{fmt(c.createdAt)}</span>
								</div>
								<p class="mt-1 whitespace-pre-wrap">{c.body}</p>
								<button
									class="mt-1 text-xs text-muted-foreground underline-offset-2 hover:underline"
									onclick={() => toggleResolve(c)}
								>
									{c.resolved ? 'Resolved · reopen' : 'Mark resolved'}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
				<form class="space-y-2" onsubmit={postComment}>
					<Textarea bind:value={draft} rows={2} placeholder="Add a comment…" />
					<Button type="submit" size="sm" disabled={posting}>
						{posting ? 'Posting…' : 'Comment'}
					</Button>
				</form>
			</div>
		{/if}
	</div>
</aside>
