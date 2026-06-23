<script lang="ts">
	// Renders one translation as text. `translationKey` names the string;
	// `fallback` is shown when it's unknown. Re-renders on SDK edits.
	import { getHijau } from './context.svelte.js';

	let { translationKey, fallback }: { translationKey: string; fallback?: string } = $props();

	const client = getHijau();
	let tick = $state(0);
	$effect(() => client.on(() => tick++));
	const text = $derived.by(() => {
		void tick; // re-derive when the SDK emits
		return client.t(translationKey, fallback);
	});
</script>

{text}
