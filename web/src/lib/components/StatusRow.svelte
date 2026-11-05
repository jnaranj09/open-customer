<script lang="ts">
	let { label, value, red = false }: { label: string; value: string; red?: boolean } = $props();
</script>

<div class="flex min-h-tap items-center justify-between gap-3 py-2">
	<span class="text-base text-ink">{label}</span>
	<span class="text-base font-bold {red ? 'text-error' : 'text-ink-muted'}">{value}</span>
</div>
