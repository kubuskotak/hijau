<script lang="ts">
	import { session } from '$lib/session.svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import Button from '$lib/components/ui/button.svelte';

	let { children } = $props();

	// Redirect to login once we know there's no authenticated user.
	$effect(() => {
		if (session.loaded && !session.user) void goto('/login');
	});

	async function logout() {
		await api.logout().catch(() => {});
		session.set(null);
		await goto('/login');
	}
</script>

{#if !session.loaded}
	<div class="grid min-h-screen place-items-center text-sm text-muted-foreground">Loading…</div>
{:else if session.user}
	<div class="flex min-h-screen flex-col">
		<header class="border-b">
			<div class="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
				<a href="/" class="font-semibold tracking-tight">Hijau</a>
				<div class="flex items-center gap-3 text-sm">
					<span class="text-muted-foreground">{session.user.email}</span>
					<Button variant="outline" size="sm" onclick={logout}>Sign out</Button>
				</div>
			</div>
		</header>
		<main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8">
			{@render children()}
		</main>
	</div>
{/if}
