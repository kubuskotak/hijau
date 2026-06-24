<script lang="ts">
	import {
		api,
		type EditorRow,
		type Language,
		type HistoryEntry,
		type Comment,
		type Screenshot,
		type TmMatch,
		type MtResult,
		type MtConfig,
		type Translation
	} from '$lib/api';
	import {Button} from '$lib/components/ui/button/index';
	import {Textarea} from '$lib/components/ui/textarea/index';
	import {Badge} from '$lib/components/ui/badge/index';
	import { stateBadge } from '$lib/translation-state';

	let {
		pid,
		keyRow,
		lang,
		baseText,
		isBase = false,
		onclose,
		onapply
	}: {
		pid: string;
		keyRow: EditorRow;
		lang: Language;
		baseText: string;
		isBase?: boolean;
		onclose: () => void;
		onapply?: (updated: Translation) => void;
	} = $props();

	type Tab = 'suggest' | 'history' | 'comments' | 'screenshots';
	let tab = $state<Tab>('history');
	let history = $state<HistoryEntry[]>([]);
	let comments = $state<Comment[]>([]);
	let screenshots = $state<Screenshot[]>([]);
	let loading = $state(true);
	let err = $state('');
	let draft = $state('');
	let posting = $state(false);

	// suggestions (target cells only)
	let tmMatches = $state<TmMatch[]>([]);
	let mtCfg = $state<MtConfig | null>(null);
	let mtResult = $state<MtResult | null>(null);
	let mtBusy = $state(false);
	let mtErr = $state('');
	let applied = $state('');

	const tabs = $derived<Tab[]>(
		isBase ? ['history', 'comments', 'screenshots'] : ['suggest', 'history', 'comments', 'screenshots']
	);

	async function loadFor(kid: string, langTag: string) {
		loading = true;
		err = '';
		try {
			[history, comments, screenshots] = await Promise.all([
				api.translationHistory(pid, kid, langTag),
				api.listComments(pid, kid, langTag),
				api.listKeyScreenshots(pid, kid)
			]);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
		// Translation memory + MT availability — target cells only.
		mtResult = null;
		mtErr = '';
		applied = '';
		if (!isBase) {
			try {
				[tmMatches, mtCfg] = await Promise.all([
					api.tmSuggest(pid, kid, langTag),
					api.mtConfig(pid)
				]);
			} catch {
				tmMatches = [];
				mtCfg = null;
			}
		}
	}

	// Reload whenever the focused cell changes; default target cells to Suggestions.
	$effect(() => {
		const cell = keyRow.id + lang.tag;
		void cell;
		tab = isBase ? 'history' : 'suggest';
		void loadFor(keyRow.id, lang.tag);
	});

	async function machineTranslate() {
		mtBusy = true;
		mtErr = '';
		try {
			mtResult = await api.mtSuggest(pid, keyRow.id, lang.tag);
		} catch (e) {
			mtErr = (e as Error).message;
		} finally {
			mtBusy = false;
		}
	}

	async function apply(text: string) {
		try {
			const updated = await api.setTranslation(pid, keyRow.id, lang.tag, text);
			onapply?.(updated);
			applied = text;
		} catch (e) {
			err = (e as Error).message;
		}
	}

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
		{#each tabs as t (t)}
			<button
				class={'border-b-2 px-3 py-2 text-sm capitalize ' +
					(tab === t
						? 'border-foreground font-medium'
						: 'border-transparent text-muted-foreground hover:text-foreground')}
				onclick={() => (tab = t)}
			>
				{t}
				{#if t === 'comments' && comments.length}({comments.length}){/if}
				{#if t === 'screenshots' && screenshots.length}({screenshots.length}){/if}
			</button>
		{/each}
	</nav>

	<div class="flex-1 overflow-y-auto p-4">
		{#if err}<p class="text-sm text-destructive">{err}</p>{/if}
		{#if tab === 'suggest'}
			<div class="space-y-4">
				<div>
					<div class="mb-1 text-xs font-medium text-muted-foreground">Translation memory</div>
					{#if tmMatches.length === 0}
						<p class="text-sm text-muted-foreground">No memory matches.</p>
					{:else}
						<ul class="space-y-2">
							{#each tmMatches as m (m.targetText)}
								<li class="rounded-md border p-2 text-sm">
									<div class="flex items-center justify-between gap-2">
										<Badge
											variant="default"
											class={m.exact
												? 'text-emerald-600 dark:text-emerald-400'
												: 'text-amber-600 dark:text-amber-500'}
										>
											{m.score}%{m.exact ? ' exact' : ''}
										</Badge>
										<Button size="sm" variant="outline" class="h-7" onclick={() => apply(m.targetText)}>
											Apply
										</Button>
									</div>
									<p class="mt-1 whitespace-pre-wrap">{m.targetText}</p>
									{#if !m.exact}
										<p class="mt-0.5 text-xs text-muted-foreground">from: {m.sourceText}</p>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</div>

				<div>
					<div class="mb-1 text-xs font-medium text-muted-foreground">Machine translation</div>
					{#if mtCfg?.enabled}
						{#if mtResult}
							<div class="rounded-md border p-2 text-sm">
								<div class="flex items-center justify-between gap-2">
									<Badge variant="secondary">{mtResult.provider}{mtResult.model ? ` · ${mtResult.model}` : ''}</Badge>
									<Button size="sm" variant="outline" class="h-7" onclick={() => apply(mtResult!.text)}>
										Apply
									</Button>
								</div>
								<p class="mt-1 whitespace-pre-wrap">{mtResult.text}</p>
							</div>
						{:else}
							<Button size="sm" variant="outline" disabled={mtBusy} onclick={machineTranslate}>
								{mtBusy ? 'Translating…' : `Machine translate (${mtCfg.provider})`}
							</Button>
						{/if}
						{#if mtErr}<p class="mt-1 text-xs text-destructive">{mtErr}</p>{/if}
					{:else}
						<p class="text-sm text-muted-foreground">
							Machine translation isn't configured for this project.
						</p>
					{/if}
				</div>

				{#if applied}
					<p class="text-xs text-emerald-600 dark:text-emerald-400">Applied “{applied}” ✓</p>
				{/if}
			</div>
		{:else if loading}
			<p class="text-sm text-muted-foreground">Loading…</p>
		{:else if tab === 'history'}
			{#if history.length === 0}
				<p class="text-sm text-muted-foreground">No history yet.</p>
			{:else}
				<ol class="space-y-3">
					{#each history as h (h.id)}
						<li class="border-l-2 pl-3 text-sm">
							<div class="flex items-center gap-2">
								{#if h.newState}{@const sb = stateBadge(h.newState)}<Badge
										variant={sb.variant}
										class={sb.class}
									>
										{sb.label}
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
		{:else if tab === 'comments'}
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
		{:else}
			{#if screenshots.length === 0}
				<p class="text-sm text-muted-foreground">
					No screenshots yet. Capture one from the in-context editor.
				</p>
			{:else}
				<div class="space-y-4">
					{#each screenshots as s (s.id)}
						<figure>
							<div class="relative overflow-hidden rounded border">
								<img src={s.imageUrl} alt={s.name} class="block w-full" />
								{#each s.regions as r (r.id)}
									{#if s.width > 0 && s.height > 0}
										<div
											class="absolute border-2 border-emerald-500 bg-emerald-500/10"
											style="left:{(r.x / s.width) * 100}%;top:{(r.y / s.height) *
												100}%;width:{(r.w / s.width) * 100}%;height:{(r.h / s.height) * 100}%"
										></div>
									{/if}
								{/each}
							</div>
							{#if s.name}
								<figcaption class="mt-1 truncate text-xs text-muted-foreground">{s.name}</figcaption>
							{/if}
						</figure>
					{/each}
				</div>
			{/if}
		{/if}
	</div>
</aside>
