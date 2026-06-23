// Shapes mirrored from the API's editContextDTO (apps/api …/editor.go) plus the
// options surface for enabling in-context editing.

export interface EditContext {
	translation: {
		id: string;
		subId: number;
		text: string;
		state: string;
		languageId: string;
		version: number;
	};
	key: { id: string; name: string; description: string; namespaceId: string };
	language: { tag: string; name: string; isRtl: boolean };
	sourceText: string;
}

export type Modifier = 'Alt' | 'Control' | 'Shift' | 'Meta';

export interface InContextOptions {
	/** API base, e.g. "/api/v1" or "https://hijau.example/api/v1". Defaults to
	 *  the value passed to createHijau(). When empty, the editor runs fully
	 *  offline against the SDK's in-memory records (great for demos/tests). */
	apiUrl?: string;
	projectId?: string;
	/** Editor token sent as `Authorization: Bearer …`. Optional when the user is
	 *  authenticated by a same-origin session cookie. */
	token?: string;
	/** Hold this key to reveal/edit translatable text. Default: Alt/Option. */
	modifier?: Modifier;
	/** Called after a successful save (server or local). */
	onSave?: (ctx: EditContext, newText: string) => void;
}
