<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import { session } from '$lib/session.svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import Label from '$lib/components/ui/label.svelte';

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		error = '';
		try {
			session.set(await api.signup({ name, email, password }));
			await goto('/');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Sign up failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="grid min-h-screen place-items-center bg-muted/30 p-4">
	<div class="w-full max-w-sm rounded-xl border bg-card p-6 shadow-sm">
		<h1 class="text-xl font-semibold tracking-tight">Create your Hijau account</h1>
		<p class="mt-1 text-sm text-muted-foreground">A workspace is set up for you automatically.</p>
		<form class="mt-6 space-y-4" onsubmit={submit}>
			<div class="space-y-1.5">
				<Label for="name">Name</Label>
				<Input id="name" bind:value={name} autocomplete="name" />
			</div>
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
					minlength={8}
					autocomplete="new-password"
				/>
				<p class="text-xs text-muted-foreground">At least 8 characters.</p>
			</div>
			{#if error}<p class="text-sm text-destructive">{error}</p>{/if}
			<Button type="submit" class="w-full" disabled={loading}>
				{loading ? 'Creating…' : 'Create account'}
			</Button>
		</form>
		<p class="mt-4 text-center text-sm text-muted-foreground">
			Already have an account? <a href="/login" class="text-foreground underline">Sign in</a>
		</p>
	</div>
</div>
