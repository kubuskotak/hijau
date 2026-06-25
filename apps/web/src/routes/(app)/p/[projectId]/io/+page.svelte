<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Language, type ImportResult, type Task, type TmxImportResult } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Textarea } from '$lib/components/ui/textarea/index';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const languages = $derived(data.languages as Language[]);

	const formats = ['json', 'json-nested', 'csv', 'android', 'apple'];

	let loading = $state(true);
	let loadErr = $state('');

	// export state
	let exFormat = $state('json');
	let exLang = $state('');
	let exState = $state('');
	const exportHref = $derived(
		api.exportUrl(pid, { format: exFormat, lang: exLang, state: exState })
	);

	// import state
	let imFormat = $state('json');
	let imLang = $state('');
	let imConflict = $state('overwrite');
	let imContent = $state('');
	let importing = $state(false);
	let importErr = $state('');
	let result = $state<ImportResult | null>(null);
	let importProgress = $state(0);
	let importPhase = $state(''); // '' | queued | running
	let importCancelled = false; // set on navigation away to stop polling

	const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

	// translation-memory (TMX) state
	let tmxExSrc = $state('');
	let tmxExTgt = $state('');
	const tmxExportHref = $derived(
		api.tmxExportUrl(pid, { sourceLang: tmxExSrc, targetLang: tmxExTgt })
	);
	let tmxContent = $state('');
	let tmxImporting = $state(false);
	let tmxErr = $state('');
	let tmxResult = $state<TmxImportResult | null>(null);

	async function onTmxFile(e: Event) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		try {
			tmxContent = await file.text();
		} catch (e) {
			tmxErr = (e as Error).message;
		}
	}

	async function doImportTMX(e: SubmitEvent) {
		e.preventDefault();
		if (!tmxContent.trim()) return;
		tmxImporting = true;
		tmxErr = '';
		tmxResult = null;
		try {
			tmxResult = await api.importTMX(pid, tmxContent);
		} catch (e) {
			tmxErr = (e as Error).message;
		} finally {
			tmxImporting = false;
		}
	}

	onMount(() => {
		try {
			const first = languages[0]?.tag ?? '';
			exLang = first;
			imLang = first;
		} catch (e) {
			loadErr = (e as Error).message;
		} finally {
			loading = false;
		}
		return () => {
			importCancelled = true; // stop any in-flight poll loop on navigation
		};
	});

	async function onFileChange(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		try {
			imContent = await file.text();
		} catch (e) {
			importErr = (e as Error).message;
		}
	}

	async function doImport(e: SubmitEvent) {
		e.preventDefault();
		if (!imLang || !imContent.trim()) return;
		importing = true;
		importErr = '';
		result = null;
		importProgress = 0;
		importPhase = '';
		try {
			// Enqueue on the server worker (async) so a large file doesn't block or
			// time out the request, then poll the task until it finishes.
			const enq = await api.importTranslations(pid, {
				format: imFormat,
				lang: imLang,
				conflict: imConflict,
				content: imContent,
				async: true
			});
			if (!enq.taskId) {
				result = enq; // server ran it inline — use the result directly
				return;
			}
			const tid = enq.taskId;
			let fails = 0;
			const maxPolls = 600; // ~7 min at 700ms before giving up
			for (let i = 0; i < maxPolls; i++) {
				await sleep(700);
				if (importCancelled) return; // navigated away
				let t: Task;
				try {
					t = await api.getTask(pid, tid);
					fails = 0;
				} catch {
					// A transient blip (proxy/restart) shouldn't kill the wait — the
					// task is still running server-side. Retry a few times first.
					if (++fails >= 5) {
						importErr = 'lost connection while waiting for the import';
						return;
					}
					continue;
				}
				importPhase = t.status;
				importProgress = t.progress;
				if (t.status === 'succeeded') {
					result = (t.result as ImportResult) ?? {
						created: 0,
						updated: 0,
						skipped: 0,
						warnings: []
					};
					return;
				}
				if (t.status === 'failed') {
					importErr = t.error || 'import failed';
					return;
				}
			}
			importErr = 'import is taking longer than expected — check back on the Tasks list';
		} catch (e) {
			importErr = (e as Error).message;
		} finally {
			importing = false;
		}
	}
</script>

<div class="grid gap-8 lg:grid-cols-2">
	<!-- EXPORT -->
	<div class="h-fit rounded-xl border bg-card p-4 shadow-sm">
		<h3 class="font-medium">Export</h3>
		<p class="text-sm text-muted-foreground">
			Download translations for one language in the chosen format.
		</p>
		{#if loading}
			<p class="mt-4 text-sm text-muted-foreground">Loading…</p>
		{:else}
			<div class="mt-3 space-y-3">
				<div class="space-y-1.5">
					<Label for="ex-format">Format</Label>
					<select
						id="ex-format"
						bind:value={exFormat}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
					>
						{#each formats as f (f)}
							<option value={f}>{f}</option>
						{/each}
					</select>
				</div>
				<div class="space-y-1.5">
					<Label for="ex-lang">Language</Label>
					<select
						id="ex-lang"
						bind:value={exLang}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
					>
						{#each languages as l (l.id)}
							<option value={l.tag}>{l.name} ({l.tag})</option>
						{/each}
					</select>
				</div>
				<div class="space-y-1.5">
					<Label for="ex-state">State filter</Label>
					<select
						id="ex-state"
						bind:value={exState}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
					>
						<option value="">Any</option>
						<option value="translated">translated</option>
						<option value="reviewed">reviewed</option>
					</select>
				</div>
				<Button href={exportHref} download data-sveltekit-reload>Download</Button>
			</div>
			{#if loadErr}<p class="mt-3 text-sm text-destructive">{loadErr}</p>{/if}
		{/if}
	</div>

	<!-- IMPORT -->
	<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={doImport}>
		<h3 class="font-medium">Import</h3>
		<p class="text-sm text-muted-foreground">
			Paste or upload a file, then import into the chosen language.
		</p>
		<div class="mt-3 space-y-3">
			<div class="grid grid-cols-2 gap-3">
				<div class="space-y-1.5">
					<Label for="im-format">Format</Label>
					<select
						id="im-format"
						bind:value={imFormat}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
					>
						{#each formats as f (f)}
							<option value={f}>{f}</option>
						{/each}
					</select>
				</div>
				<div class="space-y-1.5">
					<Label for="im-lang">Language</Label>
					<select
						id="im-lang"
						bind:value={imLang}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
					>
						{#each languages as l (l.id)}
							<option value={l.tag}>{l.name} ({l.tag})</option>
						{/each}
					</select>
				</div>
			</div>
			<div class="space-y-1.5">
				<Label for="im-conflict">On conflict</Label>
				<select
					id="im-conflict"
					bind:value={imConflict}
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
				>
					<option value="overwrite">overwrite</option>
					<option value="keep-existing">keep-existing</option>
					<option value="only-empty">only-empty</option>
				</select>
			</div>
			<div class="space-y-1.5">
				<Label for="im-file">Upload file</Label>
				<input
					id="im-file"
					type="file"
					onchange={onFileChange}
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm text-muted-foreground shadow-sm file:mr-3 file:border-0 file:bg-transparent file:text-sm file:font-medium"
				/>
			</div>
			<div class="space-y-1.5">
				<Label for="im-content">File content</Label>
				<Textarea
					id="im-content"
					bind:value={imContent}
					rows={10}
					placeholder="Paste file content here…"
					class="font-mono"
				/>
			</div>
			<Button type="submit" disabled={importing || !imLang || !imContent.trim()}>
				{#if !importing}
					Import
				{:else if importPhase === 'queued' || importPhase === ''}
					Queued…
				{:else}
					Importing… {importProgress}%
				{/if}
			</Button>
			{#if importing && importPhase === 'running'}
				<div class="h-2 w-full overflow-hidden rounded-full bg-muted">
					<div class="h-full bg-foreground transition-all" style="width: {importProgress}%"></div>
				</div>
			{/if}
		</div>

		{#if importErr}<p class="mt-3 text-sm text-destructive">{importErr}</p>{/if}

		{#if result}
			<div class="mt-4 rounded-xl border bg-card p-4 shadow-sm">
				<p class="text-sm">
					<span class="font-medium">Created:</span> {result.created} &middot;
					<span class="font-medium">Updated:</span> {result.updated} &middot;
					<span class="font-medium">Skipped:</span> {result.skipped}
				</p>
				{#if result.warnings.length > 0}
					<p class="mt-3 text-sm font-medium">Warnings</p>
					<ul class="mt-1 list-disc space-y-1 pl-5 text-sm text-muted-foreground">
						{#each result.warnings as w, i (i)}
							<li>{w}</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/if}
	</form>
</div>

<!-- TRANSLATION MEMORY (TMX) -->
<div class="mt-8 rounded-xl border bg-card p-4 shadow-sm">
	<h3 class="font-medium">Translation memory (TMX)</h3>
	<p class="text-sm text-muted-foreground">
		Move your translation memory in or out as TMX 1.4 — the interchange format used by Crowdin,
		Phrase, Tolgee and Weblate. Imported segments immediately power TM suggestions and auto-translate.
	</p>
	<div class="mt-4 grid gap-8 lg:grid-cols-2">
		<!-- TMX export -->
		<div>
			<h4 class="text-sm font-medium">Export</h4>
			<p class="mt-1 text-xs text-muted-foreground">Optionally narrow to one language pair.</p>
			<div class="mt-3 space-y-3">
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-1.5">
						<Label for="tmx-src">Source</Label>
						<select
							id="tmx-src"
							bind:value={tmxExSrc}
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
						>
							<option value="">Any</option>
							{#each languages as l (l.id)}<option value={l.tag}>{l.name} ({l.tag})</option>{/each}
						</select>
					</div>
					<div class="space-y-1.5">
						<Label for="tmx-tgt">Target</Label>
						<select
							id="tmx-tgt"
							bind:value={tmxExTgt}
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
						>
							<option value="">Any</option>
							{#each languages as l (l.id)}<option value={l.tag}>{l.name} ({l.tag})</option>{/each}
						</select>
					</div>
				</div>
				<Button href={tmxExportHref} download data-sveltekit-reload>Download .tmx</Button>
			</div>
		</div>

		<!-- TMX import -->
		<form onsubmit={doImportTMX}>
			<h4 class="text-sm font-medium">Import</h4>
			<p class="mt-1 text-xs text-muted-foreground">Upload or paste a TMX file; duplicates are skipped.</p>
			<div class="mt-3 space-y-3">
				<div class="space-y-1.5">
					<Label for="tmx-file">Upload .tmx</Label>
					<input
						id="tmx-file"
						type="file"
						accept=".tmx,.xml"
						onchange={onTmxFile}
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm text-muted-foreground shadow-sm file:mr-3 file:border-0 file:bg-transparent file:text-sm file:font-medium"
					/>
				</div>
				<div class="space-y-1.5">
					<Label for="tmx-content">TMX content</Label>
					<Textarea
						id="tmx-content"
						bind:value={tmxContent}
						rows={6}
						placeholder="Paste TMX XML here…"
						class="font-mono"
					/>
				</div>
				<Button type="submit" disabled={tmxImporting || !tmxContent.trim()}>
					{tmxImporting ? 'Importing…' : 'Import TMX'}
				</Button>
			</div>
			{#if tmxErr}<p class="mt-3 text-sm text-destructive">{tmxErr}</p>{/if}
			{#if tmxResult}
				<div class="mt-4 rounded-xl border p-3 text-sm">
					<span class="font-medium">Imported:</span>
					{tmxResult.imported} &middot; <span class="font-medium">Skipped:</span>
					{tmxResult.skipped}
					{#if tmxResult.warnings.length > 0}
						<ul class="mt-2 list-disc space-y-1 pl-5 text-muted-foreground">
							{#each tmxResult.warnings as w, i (i)}<li>{w}</li>{/each}
						</ul>
					{/if}
				</div>
			{/if}
		</form>
	</div>
</div>
