<script lang="ts">
	import { untrack } from 'svelte';
	import { api, type Language, type Translation } from '$lib/api';
	import Badge from '$lib/components/ui/badge.svelte';
	import Textarea from '$lib/components/ui/textarea.svelte';
	import Button from '$lib/components/ui/button.svelte';

	let {
		pid,
		keyId,
		lang,
		tr,
		isBase,
		onsaved,
		onopen
	}: {
		pid: string;
		keyId: string;
		lang: Language;
		tr: Translation;
		isBase: boolean;
		onsaved: (updated: Translation, wasBase: boolean) => void;
		onopen?: () => void;
	} = $props();

	// Local editable copy; initialised from the prop (intentionally not tracked
	// here — the $effect below re-syncs when the translation changes).
	let draft = $state(untrack(() => tr.text));
	let saving = $state(false);
	let err = $state('');

	// Re-sync the draft when the translation changes externally (e.g. a refetch
	// after a base-language edit cascades OUTDATED to this cell).
	$effect(() => {
		draft = tr.text;
	});

	async function save() {
		if (draft === tr.text || saving) return;
		saving = true;
		err = '';
		try {
			onsaved(await api.setTranslation(pid, keyId, lang.tag, draft), isBase);
		} catch (e) {
			err = (e as Error).message;
			draft = tr.text;
		} finally {
			saving = false;
		}
	}

	async function approve() {
		saving = true;
		err = '';
		try {
			onsaved(await api.transition(pid, keyId, lang.tag, 'approve'), false);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
			e.preventDefault();
			(e.currentTarget as HTMLTextAreaElement).blur();
		}
	}

	const canApprove = $derived(
		!isBase &&
			tr.text.trim() !== '' &&
			(tr.state === 'translated' || tr.state === 'outdated' || tr.state === 'needs_work')
	);
</script>

<div class="space-y-1">
	<Textarea
		bind:value={draft}
		rows={1}
		onblur={save}
		{onkeydown}
		disabled={saving}
		dir={lang.isRtl ? 'rtl' : 'ltr'}
		placeholder="—"
	/>
	<div class="flex items-center gap-2">
		<Badge variant={tr.state}>{tr.state.replace('_', ' ')}</Badge>
		{#if canApprove}
			<Button size="sm" variant="ghost" class="h-6 px-2 text-xs" onclick={approve} disabled={saving}>
				Approve
			</Button>
		{/if}
		{#if onopen}
			<button
				type="button"
				class="ml-auto rounded px-1 text-muted-foreground hover:text-foreground"
				onclick={() => onopen?.()}
				title="History & comments"
				aria-label="History & comments"
			>
				⋯
			</button>
		{/if}
		{#if err}<span class="text-xs text-destructive">{err}</span>{/if}
	</div>
</div>
