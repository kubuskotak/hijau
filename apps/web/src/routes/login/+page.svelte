<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import { session } from '$lib/session.svelte';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input} from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		error = '';
		try {
			session.set(await api.login({ email, password }));
			await goto('/');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Sign in failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="grid min-h-screen place-items-center bg-muted/30 p-4">
	<div class="w-full max-w-sm rounded-xl border bg-card p-6 shadow-sm">
		<h1 class="text-xl font-semibold tracking-tight">Sign in to Hijau</h1>
		<p class="mt-1 text-sm text-muted-foreground">Welcome back.</p>
		<form class="mt-6 space-y-4" onsubmit={submit}>
			<div class="space-y-1.5">
				<Label for="email">Email</Label>
				<Input id="email" type="email" bind:value={email} required autocomplete="email" />
			</div>
			<div class="space-y-1.5">
				<Label for="password">Password</Label>
				<Input
					id="password"
					type="password"
					bind:value={password}
					required
					autocomplete="current-password"
				/>
			</div>
			{#if error}<p class="text-sm text-destructive">{error}</p>{/if}
			<Button type="submit" class="w-full" disabled={loading}>
				{loading ? 'Signing in…' : 'Sign in'}
			</Button>
		</form>
		<p class="mt-4 text-center text-sm text-muted-foreground">
			No account? <a href="/signup" class="text-foreground underline">Create one</a>
		</p>
	</div>
</div>
