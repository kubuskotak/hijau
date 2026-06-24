<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import { session } from '$lib/session.svelte';

	let { children } = $props();

	onMount(() => {
		void session.load();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Hijau</title>
</svelte:head>

{@render children()}
