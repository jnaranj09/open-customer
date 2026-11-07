<script lang="ts">
	let { name }: { name: string } = $props();
</script>

<!-- ruleid: svelte-no-html-tag -->
<p>{@html name}</p>

<!-- ruleid: svelte-no-html-tag -->
<p>{@html '<b>' + name + '</b>'}</p>

<!-- ok: svelte-no-html-tag -->
<p>{name}</p>

<!-- ok: svelte-no-html-tag -->
<p>html is written here only as text</p>
