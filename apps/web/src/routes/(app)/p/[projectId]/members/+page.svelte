<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Member, type Language } from '$lib/api';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Badge } from '$lib/components/ui/badge/index';

	let { data } = $props();
	const pid = $derived(data.project.id);
	const languages = $derived(data.languages as Language[]);

	const ROLES = ['owner', 'admin', 'developer', 'translator', 'reviewer'];
	const selectClass =
		'flex h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm';

	let members = $state<Member[]>([]);
	let loading = $state(true);
	let err = $state('');
	let email = $state('');
	let role = $state('translator');
	let adding = $state(false);

	async function load() {
		loading = true;
		err = '';
		try {
			members = await api.listMembers(pid);
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	async function add(e: SubmitEvent) {
		e.preventDefault();
		if (!email.trim()) return;
		adding = true;
		err = '';
		try {
			members = [...members, await api.addMember(pid, { email: email.trim(), role })];
			email = '';
		} catch (e) {
			err = (e as Error).message;
		} finally {
			adding = false;
		}
	}

	async function changeRole(m: Member, newRole: string) {
		try {
			await api.updateMemberRole(pid, m.id, newRole);
			m.role = newRole;
		} catch (e) {
			err = (e as Error).message;
		}
	}

	async function remove(m: Member) {
		try {
			await api.removeMember(pid, m.id);
			members = members.filter((x) => x.id !== m.id);
		} catch (e) {
			err = (e as Error).message;
		}
	}

	function toggleLang(m: Member, langId: string) {
		m.languageIds = m.languageIds.includes(langId)
			? m.languageIds.filter((x) => x !== langId)
			: [...m.languageIds, langId];
	}
	async function saveLangs(m: Member) {
		try {
			await api.setMemberLanguages(pid, m.id, m.languageIds);
		} catch (e) {
			err = (e as Error).message;
		}
	}
	const scoped = (r: string) => r === 'translator' || r === 'reviewer';
</script>

<div class="grid gap-8 lg:grid-cols-[1fr_20rem]">
	<div>
		<h2 class="text-lg font-medium">Members</h2>
		<p class="text-sm text-muted-foreground">
			Who can access this project and what they can do. Translators and reviewers can be limited
			to specific languages.
		</p>

		{#if err}<p class="mt-3 text-sm text-destructive">{err}</p>{/if}

		{#if loading}
			<p class="mt-6 text-sm text-muted-foreground">Loading…</p>
		{:else if members.length === 0}
			<div class="mt-6 rounded-xl border border-dashed p-10 text-center">
				<p class="text-sm text-muted-foreground">
					No direct members yet — add a teammate by email.
				</p>
			</div>
		{:else}
			<ul class="mt-4 space-y-3">
				{#each members as m (m.id)}
					<li class="rounded-xl border bg-card p-4 shadow-sm">
						<div class="flex items-center justify-between gap-3">
							<div class="min-w-0">
								<div class="truncate font-medium">{m.name || m.email}</div>
								<div class="truncate text-xs text-muted-foreground">{m.email}</div>
							</div>
							<div class="flex items-center gap-2">
								<select
									class={selectClass + ' w-32'}
									value={m.role}
									onchange={(e) => changeRole(m, e.currentTarget.value)}
								>
									{#each ROLES as r (r)}<option value={r}>{r}</option>{/each}
								</select>
								<Button variant="outline" size="sm" onclick={() => remove(m)}>Remove</Button>
							</div>
						</div>

						{#if scoped(m.role) && languages.length > 0}
							<div class="mt-3 border-t pt-3">
								<div class="mb-1 text-xs font-medium text-muted-foreground">
									Languages (empty = all)
								</div>
								<div class="flex flex-wrap items-center gap-3">
									{#each languages as l (l.id)}
										<label class="flex items-center gap-1.5 text-sm">
											<input
												type="checkbox"
												class="h-4 w-4 rounded border-input"
												checked={m.languageIds.includes(l.id)}
												onchange={() => toggleLang(m, l.id)}
											/>
											{l.tag}
										</label>
									{/each}
									<Button variant="outline" size="sm" class="h-7" onclick={() => saveLangs(m)}>
										Save scope
									</Button>
								</div>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	<form class="h-fit rounded-xl border bg-card p-4 shadow-sm" onsubmit={add}>
		<h3 class="font-medium">Add a member</h3>
		<p class="mt-1 text-xs text-muted-foreground">The person must already have an account.</p>
		<div class="mt-3 space-y-3">
			<div class="space-y-1.5">
				<Label for="m-email">Email</Label>
				<Input id="m-email" type="email" bind:value={email} placeholder="teammate@example.com" required />
			</div>
			<div class="space-y-1.5">
				<Label for="m-role">Role</Label>
				<select id="m-role" class={selectClass} bind:value={role}>
					{#each ROLES as r (r)}<option value={r}>{r}</option>{/each}
				</select>
			</div>
			<Button type="submit" disabled={adding}>{adding ? 'Adding…' : 'Add member'}</Button>
		</div>
		<div class="mt-4 border-t pt-3 text-xs text-muted-foreground">
			<Badge variant="secondary">owner/admin</Badge> manage the project ·
			<Badge variant="secondary">developer</Badge> keys + all translations ·
			<Badge variant="secondary">translator</Badge> translate (scoped) ·
			<Badge variant="secondary">reviewer</Badge> approve (scoped)
		</div>
	</form>
</div>
