import type { LayoutLoad } from './$types';
import { api } from '$lib/api';

export const load: LayoutLoad = async ({ params }) => {
	const projectId = params.projectId;
	const [project, languages] = await Promise.all([
		api.getProject(projectId),
		api.listLanguages(projectId)
	]);
	return { project, languages };
};
