<script lang="ts" module>
	import { LIVE_LAYER, inertBehind, type Layer } from '../../inert';
	import { createSheetStack } from '../../sheetStack';

	// Every open sheet, shared by all instances: one page lock, one top sheet for Escape and Tab,
	// and everything behind the top sheet inert.
	let savedOverflow = '';
	const madeInert = new Set<Layer>();
	const sheets = createSheetStack<HTMLElement>({
		lock: () => {
			savedOverflow = document.body.style.overflow;
			document.body.style.overflow = 'hidden';
		},
		unlock: () => {
			document.body.style.overflow = savedOverflow;
		},
		showTop: (top) => {
			const layers = [...document.body.children].filter((el) => el instanceof HTMLElement);
			inertBehind(layers, top, madeInert);
		}
	});
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { nextFocus } from '../../focus';
	import { t } from '../../i18n';
	import { toBody } from '../../inert';

	let {
		open,
		title,
		onclose,
		dismissable = true,
		children
	}: {
		open: boolean;
		title: string;
		onclose: () => void;
		/** False for a question she must answer with a button: Escape, a tap outside and a swipe do nothing. */
		dismissable?: boolean;
		children: Snippet;
	} = $props();

	const titleId = $props.id();
	const FOCUSABLE =
		'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

	// The sheet's own layer of <body>, and the panel inside it.
	let layer: HTMLElement | undefined = $state();
	let panel: HTMLElement | undefined = $state();
	let dragY = $state(0);
	let dragStart: number | null = null;

	// The controls of a live layer (the toast's "Deshacer") stay out of the inert page, so the Tab
	// cycle reaches them too, after the sheet's own.
	const LIVE_FOCUSABLE = FOCUSABLE.split(', ')
		.map((sel) => `[${LIVE_LAYER}] ${sel}`)
		.join(', ');

	function focusables(): HTMLElement[] {
		return panel === undefined
			? []
			: [
					...panel.querySelectorAll<HTMLElement>(FOCUSABLE),
					...document.querySelectorAll<HTMLElement>(LIVE_FOCUSABLE)
				];
	}

	// Focus moves into the sheet when it opens and back to what had it when it closes.
	$effect(() => {
		if (!open || panel === undefined || layer === undefined) {
			return;
		}
		const before = document.activeElement;
		const own = layer;
		sheets.push(own);
		(focusables()[0] ?? panel).focus();
		return () => {
			sheets.remove(own);
			if (before instanceof HTMLElement) {
				before.focus();
			}
		};
	});

	function onkeydown(event: KeyboardEvent): void {
		if (!open || layer === undefined || !sheets.isTop(layer)) {
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			if (dismissable) {
				onclose();
			}
			return;
		}
		if (event.key !== 'Tab') {
			return;
		}
		const items = focusables();
		const target = nextFocus(
			items.findIndex((el) => el === document.activeElement),
			items.length,
			event.shiftKey
		);
		event.preventDefault();
		(items[target] ?? panel)?.focus();
	}

	function dragDown(event: PointerEvent): void {
		dragStart = event.clientY;
		if (event.currentTarget instanceof HTMLElement) {
			event.currentTarget.setPointerCapture(event.pointerId);
		}
	}

	function dragMove(event: PointerEvent): void {
		if (dragStart !== null) {
			dragY = Math.max(0, event.clientY - dragStart);
		}
	}

	function dragEnd(): void {
		if (dragStart === null) {
			return;
		}
		// A swipe down past a quarter of the sheet's height closes it; less springs back.
		const close = dismissable && panel !== undefined && dragY > panel.offsetHeight / 4;
		dragStart = null;
		dragY = 0;
		if (close) {
			onclose();
		}
	}
</script>

<svelte:window {onkeydown} />

{#if open}
	<div
		{@attach toBody}
		bind:this={layer}
		class="fixed inset-0 z-50 flex items-end justify-center lg:items-center"
	>
		<button
			type="button"
			tabindex="-1"
			aria-label={t('sheet_close')}
			disabled={!dismissable}
			class="absolute inset-0 cursor-default bg-ink/30"
			onclick={onclose}
		></button>
		<div
			bind:this={panel}
			role="dialog"
			aria-modal="true"
			aria-labelledby={titleId}
			tabindex="-1"
			style:transform={dragY > 0 ? `translateY(${String(dragY)}px)` : undefined}
			class="glass-panel shadow-sheet relative flex max-h-sheet w-full flex-col items-center gap-4 overflow-y-auto rounded-t-xl px-6 pt-3 lg:max-w-120 lg:rounded-xl lg:pt-6"
		>
			<!-- The swipe handle is a touch affordance; the Escape key and the scrim close it too. -->
			<div
				class="flex h-6 w-full shrink-0 touch-none items-center justify-center lg:hidden {dismissable
					? ''
					: 'invisible'}"
				onpointerdown={dragDown}
				onpointermove={dragMove}
				onpointerup={dragEnd}
				onpointercancel={dragEnd}
				role="presentation"
			>
				<div class="h-1 w-10 rounded-pill bg-ink-muted/40"></div>
			</div>
			<h2 id={titleId} class="w-full text-xl font-bold text-ink">{title}</h2>
			<div class="flex w-full flex-col gap-3 pb-8">
				{@render children()}
			</div>
			<div class="pb-safe"></div>
		</div>
	</div>
{/if}
