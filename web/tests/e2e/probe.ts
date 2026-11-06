// The measurements behind `ui-probe.spec.ts` (.claude/rules/ui.md, stage 7): axe, and the layout
// checks no package does (sideways scroll, tap targets, input font size, cut text, the page behind
// an open sheet). Each one returns findings; the spec drops the allow-listed ones and fails on the
// rest, and on an allow-list entry that matched nothing.
import AxeBuilder from '@axe-core/playwright';
import type { Page } from '@playwright/test';

/** One word, 60 letters: the longest kind of name a layout has to hold. Made up; the probe spec
 *  seeds it and the allow-list names the boxes it breaks. */
export const LONG_NAME = 'Mariafernanda'.repeat(5).slice(0, 60);

/** The two probed widths: the phone profile and the main desktop size. */
export type Width = 'compact' | 'wide';

/** One problem on one screen. `target` names the element: its tag and its whole text (`span "…"`),
 *  axe's selector, `document`, or the data feed. */
export type Finding = { check: string; target: string; detail: string };

/** A known finding that waits on a fix: `bug` is its id (`UIP-…`) and `why` says what the bug
 *  is. Without `width`, `state`, `target` or `detail` the entry matches any. `route: '*'` allows
 *  the finding on every route and every state (it takes no `state`); it must still show on its
 *  `witness` route (base state), or it is stale. */
export type Allow = {
	check: string;
	width?: Width;
	/** The screen state: `base`, an opened sheet's name, or a data feed's name and `error`,
	 *  `busy` or `empty` (`sent log busy`). */
	state?: string;
	/** The finding's target, exactly. */
	target?: string;
	/** A substring of the finding's detail. */
	detail?: string;
	why: string;
	bug: string;
} & ({ route: string } | { route: '*'; witness: string });

/** Every WCAG 2.0, 2.1 and 2.2 A/AA rule. `target-size` (24 px) carries only `wcag22aa`, so it runs
 *  only because that tag is listed. */
const AXE_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

/** axe on the whole page. Throws when `target-size` did not run, so a tag change cannot drop it. */
export async function axe(page: Page): Promise<Finding[]> {
	const results = await new AxeBuilder({ page }).withTags(AXE_TAGS).analyze();
	const ran = [
		...results.violations,
		...results.passes,
		...results.incomplete,
		...results.inapplicable
	].some((r) => r.id === 'target-size');
	if (!ran) {
		throw new Error('axe: target-size did not run');
	}
	return results.violations.flatMap((v) =>
		v.nodes.map((n) => ({
			check: `axe:${v.id}`,
			target: n.target.join(' '),
			detail: n.any[0]?.message ?? v.help
		}))
	);
}

/** The smallest tap target (.claude/rules/ui.md "Accessibility and touch basics"). */
const TARGET_PX = 44;
/** iOS zooms the page on focus of a field under this size. */
const FIELD_FONT_PX = 16;

/** The layout checks on the page as it is now. Runs in the browser: no outside names. */
function measureInPage(limits: { target: number; font: number }): Finding[] {
	const out: Finding[] = [];
	const norm = (text: string): string => text.replace(/\s+/g, ' ').trim();
	const name = (el: Element): string => {
		const field = el instanceof HTMLInputElement || el instanceof HTMLSelectElement;
		const fieldLabel = field ? el.labels?.[0]?.textContent : undefined;
		const label = el.getAttribute('aria-label') ?? fieldLabel ?? el.textContent;
		return `${el.tagName.toLowerCase()} "${norm(label)}"`;
	};
	// Drawn boxes only: not display:none, not visibility:hidden, not a 1 px screen-reader-only box.
	const drawn = (el: Element): boolean => {
		const r = el.getBoundingClientRect();
		const s = getComputedStyle(el);
		return r.width > 1 && r.height > 1 && s.visibility !== 'hidden';
	};

	const root = document.documentElement;
	if (root.scrollWidth > root.clientWidth) {
		out.push({
			check: 'scroll',
			target: 'document',
			detail: `scrollWidth ${String(root.scrollWidth)} > clientWidth ${String(root.clientWidth)}`
		});
	}

	const CONTROLS =
		'a[href], button, input:not([type="hidden"]), select, textarea, summary, [role="button"], [role="link"], [tabindex]:not([tabindex="-1"])';
	for (const el of document.querySelectorAll(CONTROLS)) {
		// A field inside its label is tapped through the label.
		const box = el instanceof HTMLInputElement ? (el.closest('label') ?? el) : el;
		if (!drawn(box)) continue;
		const r = box.getBoundingClientRect();
		if (r.width < limits.target - 0.5 || r.height < limits.target - 0.5) {
			out.push({
				check: 'target-size',
				target: name(el),
				detail: `${String(Math.round(r.width))}x${String(Math.round(r.height))}`
			});
		}
	}

	const NOT_TYPED = ['checkbox', 'radio', 'range', 'color', 'file', 'submit', 'button', 'reset'];
	for (const el of document.querySelectorAll('input, select, textarea')) {
		if (el instanceof HTMLInputElement && (el.type === 'hidden' || NOT_TYPED.includes(el.type))) {
			continue;
		}
		const size = parseFloat(getComputedStyle(el).fontSize);
		if (size < limits.font) {
			out.push({ check: 'font-size', target: name(el), detail: `${String(size)}px` });
		}
	}

	for (const el of document.body.querySelectorAll('*')) {
		if (!drawn(el)) continue;
		const s = getComputedStyle(el);
		if (s.display === 'inline' || s.display === 'contents') continue;
		const cutsX = s.overflowX === 'hidden' || s.overflowX === 'clip';
		const cutsY = s.overflowY === 'hidden' || s.overflowY === 'clip';
		const wide = el.scrollWidth > el.clientWidth + 1;
		const tall = el.scrollHeight > el.clientHeight + 1;
		// A one-line ellipsis is a deliberate cut: the "…" shows it.
		const ellipsis =
			s.textOverflow === 'ellipsis' &&
			(s.whiteSpace === 'nowrap' || s.getPropertyValue('text-wrap-mode') === 'nowrap');
		if ((cutsX && wide && !ellipsis) || (cutsY && tall)) {
			out.push({
				check: 'clip',
				target: name(el),
				detail: `content ${String(el.scrollWidth)}x${String(el.scrollHeight)} in ${String(el.clientWidth)}x${String(el.clientHeight)}`
			});
		}
	}

	// Text wider than the block box it sits in, inline elements included: each text node's own
	// rectangle against its nearest block's content width. A block that clips or scrolls is left to
	// the clip check above.
	const blockOf = (n: Node): Element | null => {
		let e = n.parentElement;
		while (e !== null) {
			const d = getComputedStyle(e).display;
			if (d !== 'inline' && d !== 'contents') return e;
			e = e.parentElement;
		}
		return null;
	};
	const spilled = new Set<Element>();
	const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
	for (let n = walker.nextNode(); n !== null; n = walker.nextNode()) {
		const text = norm(n.textContent ?? '');
		const block = blockOf(n);
		if (text === '' || block === null || spilled.has(block) || !drawn(block)) continue;
		if (getComputedStyle(block).overflowX !== 'visible') continue;
		const range = document.createRange();
		range.selectNodeContents(n);
		const r = range.getBoundingClientRect();
		const left = block.getBoundingClientRect().left + block.clientLeft;
		const right = left + block.clientWidth;
		if (r.width > 0 && (r.right > right + 1 || r.left < left - 1)) {
			spilled.add(block);
			out.push({
				check: 'text-overflow',
				target: `${(n.parentElement ?? block).tagName.toLowerCase()} "${text}"`,
				detail: `text ${String(Math.round(r.width))} wide in ${String(block.clientWidth)}`
			});
		}
	}
	return out;
}

/** Sideways scroll, tap targets, field font size and cut or spilling text. */
export function layout(page: Page): Promise<Finding[]> {
	return page.evaluate(measureInPage, { target: TARGET_PX, font: FIELD_FONT_PX });
}

/** With a dialog open, everything outside it must be inert (or hidden from assistive tech). */
function behindInPage(): Finding[] {
	const dialog = document.querySelector('[role="dialog"]');
	if (dialog === null) {
		return [{ check: 'inert', target: 'dialog', detail: 'no open dialog' }];
	}
	return [...document.body.children]
		.filter((el) => !el.contains(dialog) && el.tagName !== 'SCRIPT')
		.filter(
			(el) =>
				!(el instanceof HTMLElement && el.inert) && el.closest('[aria-hidden="true"]') === null
		)
		.map((el) => ({
			check: 'inert',
			target: el.tagName.toLowerCase(),
			detail: 'page behind the open sheet is not inert'
		}));
}

export function behindSheet(page: Page): Promise<Finding[]> {
	return page.evaluate(behindInPage);
}

type Scope = { route: string; width: Width; state: string };

/** Whether `a` is about this screen: it allows findings here. */
function applies(a: Allow, s: Scope): boolean {
	return (
		(a.route === '*' || a.route === s.route) &&
		(a.width === undefined || a.width === s.width) &&
		(a.route === '*' || (a.state ?? 'base') === s.state)
	);
}

/** Whether `a` must match a finding on this screen, or be stale. */
function expected(a: Allow, s: Scope): boolean {
	const home = 'witness' in a ? a.witness === s.route && s.state === 'base' : true;
	return home && applies(a, s);
}

function covers(a: Allow, f: Finding): boolean {
	return (
		a.check === f.check &&
		(a.target === undefined || f.target === a.target) &&
		(a.detail === undefined || f.detail.includes(a.detail))
	);
}

/** The findings no entry allows, and the entries for this screen that allowed nothing (stale:
 *  delete them). */
export function sift(
	findings: Finding[],
	allow: readonly Allow[],
	scope: Scope
): { open: Finding[]; stale: Allow[] } {
	const mine = allow.filter((a) => applies(a, scope));
	return {
		open: findings.filter((f) => !mine.some((a) => covers(a, f))),
		stale: allow.filter((a) => expected(a, scope) && !findings.some((f) => covers(a, f)))
	};
}
