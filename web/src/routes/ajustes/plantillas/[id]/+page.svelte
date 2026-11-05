<script lang="ts">
	import { page } from '$app/state';
	import { cache } from '../../../../lib/cache/store.svelte';
	import TemplatePane from '../../../../lib/components/TemplatePane.svelte';

	const template = $derived(cache.snapshot?.templates.find((x) => x.id === page.params['id']));
</script>

<!-- Keyed on the id, so another template starts with its own name and text. -->
{#key page.params['id']}
	{#if template === undefined}
		<TemplatePane missing />
	{:else}
		<TemplatePane {template} />
	{/if}
{/key}
