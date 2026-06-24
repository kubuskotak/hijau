// Reactive session state (Svelte 5 runes in a module).
import { api, type User } from '$lib/api';

let user = $state<User | null>(null);
let loaded = $state(false);

export const session = {
	get user() {
		return user;
	},
	get loaded() {
		return loaded;
	},
	async load() {
		try {
			user = await api.me();
		} catch {
			user = null;
		} finally {
			loaded = true;
		}
	},
	set(u: User | null) {
		user = u;
		loaded = true;
	}
};
